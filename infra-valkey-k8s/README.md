# infra-valkey-k8s

Despliegue **100% Ansible** de **Valkey** (replicación + Sentinel) sobre un clúster
**k3s** de nodo único, más una app Go de ejemplo que demuestra *cache-aside*.

## Arquitectura

- Este clúster K8s tiene **su propia** instancia de Valkey, que sirve **solo** a
  apps dentro del mismo clúster. No se expone fuera (sin NodePort/LoadBalancer).
- Cachés desacopladas: otros VPS con Docker Swarm tendrán cada uno su Valkey local
  (eso se automatiza en otro repo/role — ver "Próximos pasos").
- Valkey: 3 pods (1 primary + 2 réplicas), 3 Sentinels (quorum 2), **sin
  persistencia** (caché puro, `maxmemory` + `allkeys-lru`).
- Namespaces: `data` (Valkey) y `production` (apps).
- **Asunción:** Prometheus Operator **NO** instalado (`prometheus_operator_installed: false`).
  Pon la variable a `true` si instalas kube-prometheus-stack y quieres `ServiceMonitor`.

---

## 1. Despliegue rápido (un archivo, un comando)

Todo lo que sueles editar vive en **un único archivo**: `group_vars/all/config.yml`.
El resto (instalar tooling, traer el kubeconfig, generar el password) lo hacen
los targets del `Makefile`.

```bash
cd infra-valkey-k8s

make setup     # crea el .venv e instala Ansible + collection (una sola vez)
make config    # crea group_vars/all/config.yml desde la plantilla

# >>> edita group_vars/all/config.yml <<<
#     Para Valkey solo, basta con 2 campos: node_ip y ssh_key.

make deploy    # despliega Valkey (+ verificación). La app de ejemplo es opcional.
```

Eso es todo. `make deploy` es idempotente: re-ejecútalo cuando cambies la config.

### Qué hay en `config.yml`

| Campo | Obligatorio | Qué es |
|-------|:-----------:|--------|
| `node_ip` | ✅ | IP pública del VPS k3s |
| `ssh_key` | ✅ | ruta a tu clave SSH privada |
| `node_user` | | usuario SSH (default `root`) |
| `kubeconfig` | | ruta al kubeconfig local. Si ya tienes uno, apúntalo aquí y la copia se salta |
| `valkey_namespace` | | namespace de Valkey (default `data`) |
| `apps_namespace` | | namespace de tus apps que consumen la caché (default `production`; pon `default` si despliegas ahí) |
| `valkey_password` | | **déjalo vacío** y se autogenera uno fuerte (en `.valkey_password`, gitignored) |
| `valkey_maxmemory_mb` | | caché útil por instancia (MB). El límite del pod se deriva solo (~1.3x). Recuerda: es replicación, no se suma entre pods |
| `valkey_chart_version`, `app_replicas`, ... | | toggles con defaults razonables |
| `deploy_app_ejemplo` | | `false` por defecto. Ponlo a `true` para desplegar la demo de cache-aside |
| `app_image` | sólo si la demo | imagen de la app (fijada por tag/digest), obligatoria solo con `deploy_app_ejemplo: true` |

> No necesitas `openssl`, ni `ansible-vault`, ni copiar el kubeconfig a mano, ni
> editar el inventario: todo se deriva de `config.yml`.
>
> **La app de ejemplo es solo una demo** del patrón cache-aside. Valkey funciona
> y queda listo para tus apps sin desplegarla. Actívala (`deploy_app_ejemplo: true`)
> únicamente si quieres ver el demo — requiere construir/publicar su imagen.

> **¿No usas k3s?** El despliegue es Kubernetes estándar (Helm + manifiestos); lo
> único atado a k3s es de dónde se copia el kubeconfig. Si **ya tienes un kubeconfig
> que funciona**, apunta `kubeconfig:` a él y la copia se salta sola. Si usas otra
> distro y aún no tienes kubeconfig local, define `node_kubeconfig_src` con la ruta
> del kubeconfig en el nodo.

### Targets disponibles

```
make help      # lista los targets
make setup     # .venv + Ansible + libs + collection kubernetes.core
make config    # crea group_vars/all/config.yml desde la plantilla
make deploy    # despliega todo (site.yml)
make verify    # re-ejecuta solo la verificación post-deploy
make clean     # desinstala Valkey, la app y los namespaces del clúster
make reset     # borra .venv y collections locales (no toca el clúster)
```

### Herramientas externas

- **Helm 3** en el nodo: lo instala `00-prereqs.yml` si falta.
- **kubectl** no es necesario en el laptop (Ansible habla por API), pero ayuda a depurar.

---

## 2. Qué hace `make deploy` por dentro

`make deploy` ejecuta `site.yml`, que encadena estos playbooks (todos idempotentes):

> - **00-prereqs**: instala Helm 3 en el nodo si falta; registra/actualiza el repo `bitnami`.
> - **05-fetch-kubeconfig**: copia el kubeconfig de k3s a tu laptop y reescribe el
>   `server:` apuntando a `node_ip` (solo si no existe ya localmente).
> - **10-valkey-deploy**: crea `data` y `production` (etiqueta `name=production`),
>   crea el Secret `valkey-auth`, renderiza `valkey-values.yaml.j2`, instala el chart
>   `bitnami/valkey` (versión fija), aplica la NetworkPolicy y espera a los 3 pods Ready.
> - **20-app-deploy** (opcional): solo si `deploy_app_ejemplo: true`; replica
>   `valkey-auth` a `production` y despliega la app de demo.
> - **99-verify**: comprueba Valkey/Sentinel y **falla** si algo no cuadra. Las
>   comprobaciones de la app solo corren si la demo está activada.

### Ejecución manual (sin Makefile)

Si prefieres no usar `make`, con el `.venv` activado:

```bash
source .venv/bin/activate
cp config.example.yml group_vars/all/config.yml   # edítalo
ansible-playbook site.yml                          # equivale a `make deploy`
```

### Sobre el password y el cifrado

Por defecto el password se autogenera y se guarda en claro en `.valkey_password`
(gitignored), pensado para un único operador. Si quieres cifrado en reposo, crea
un vault y deja `valkey_password` vacío en `config.yml`:

```bash
ansible-vault create group_vars/all/vault.yml   # define vault_valkey_password
ansible-playbook site.yml --ask-vault-pass       # (o --vault-password-file)
```

El `vault.yml`, si existe, tiene prioridad sobre el password de `config.yml`.

---

## 4. Rollback

Vía Helm (Ansible):

```bash
# Lista revisiones
helm --kubeconfig ~/.kube/k3s-valkey.yaml history valkey -n data

# Rollback a la revisión N (ej. 1)
helm --kubeconfig ~/.kube/k3s-valkey.yaml rollback valkey 1 -n data
```

O dentro de un playbook ad-hoc con el módulo `kubernetes.core.helm` + `chart_version`
de la revisión anterior, o con `command: helm rollback ...` envuelto puntualmente.

---

## 5. Desinstalación limpia

```bash
make clean      # equivale a: ansible-playbook playbooks/99-cleanup.yml
```

Borra (idempotente) la app, el release de Helm de Valkey, los Secrets, la
NetworkPolicy y los namespaces `data` y `production`.

> No hay PVCs que limpiar: la persistencia está desactivada.

Equivalente manual con `kubectl`/`helm` si lo prefieres:

```bash
kubectl --kubeconfig ~/.kube/k3s-valkey.yaml delete -n production \
  deploy/app-ejemplo svc/app-ejemplo secret/valkey-auth --ignore-not-found
helm --kubeconfig ~/.kube/k3s-valkey.yaml uninstall valkey -n data
kubectl --kubeconfig ~/.kube/k3s-valkey.yaml delete -n data \
  secret/valkey-auth netpol/valkey-allow-clients-and-internal --ignore-not-found
kubectl --kubeconfig ~/.kube/k3s-valkey.yaml delete ns data production
```

---

## 6. Troubleshooting (los más comunes)

1. **Pods en `Pending` con anti-afinidad `hard`.**
   En nodo único no hay 3 nodos donde repartir los pods. Usa
   `valkey_pod_anti_affinity_preset: soft` (default). Cambia a `hard` solo con
   ≥3 nodos worker.

2. **`ImagePullBackOff` en los pods de Valkey.**
   Desde mediados de 2025 Bitnami movió las imágenes "free tier" al repo
   `bitnamilegacy` (catálogo "Bitnami Secure Images"). Si el pull falla,
   descomenta en `valkey-values.yaml.j2` el bloque `image:` apuntando a
   `bitnamilegacy/valkey` (y análogos para `sentinel`/`metrics`), o fija una
   versión de chart cuyas imágenes sigan disponibles. Verifica versiones con
   `helm search repo bitnami/valkey --versions`.

3. **La NetworkPolicy "no hace nada".**
   El CNI por defecto de k3s (**flannel**) **no** aplica NetworkPolicies. Instala
   Calico (k3s con `--flannel-backend=none --disable-network-policy=false` + Calico,
   o tu CNI con soporte de NetworkPolicy) para que las reglas surtan efecto.
   Sin un CNI compatible, las reglas se crean pero se ignoran.

4. **Módulos `kubernetes.core` fallan con `Failed to import the required Python library (kubernetes)`.**
   No corriste `make setup` (o usas un Python sin las libs). Re-ejecuta
   `make setup`; los targets usan el `ansible-playbook` del `.venv` directamente.

5. **`node_ip is undefined` / el inventario no resuelve la conexión.**
   Falta `group_vars/all/config.yml`. Ejecuta `make config` y edítalo (el
   inventario lee `node_ip`/`ssh_key` de ahí).

6. **Quiero re-traer el kubeconfig del nodo.**
   `05-fetch-kubeconfig` no sobrescribe si ya existe. Borra el archivo local
   (`rm ~/.kube/k3s-valkey.yaml`) y vuelve a `make deploy`.

7. **`--ask-vault-pass` molesta (solo si usas vault opcional).**
   Usa `--vault-password-file .vault-pass` (añádelo a `.gitignore`) o
   `ANSIBLE_VAULT_PASSWORD_FILE`.

---

Ver `CHECKLIST.md` para la lista de verificación por fases (incluido el test de failover).
