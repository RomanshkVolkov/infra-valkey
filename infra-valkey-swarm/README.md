# infra-valkey-swarm

Despliegue **100% Ansible** de **Valkey standalone** (caché puro) en uno o varios
**VPS con Docker Swarm**. Cada VPS tiene su **propia** instancia local de Valkey;
**no hay caché compartida** entre VPS ni con el clúster K8s (repo `infra-valkey-k8s`).

## Diferencias clave frente al repo de K8s

| | K8s (`infra-valkey-k8s`) | Swarm (este repo) |
|---|---|---|
| Topología | 1 primary + 2 réplicas + 3 Sentinels | **1 instancia standalone por VPS** |
| Alta disponibilidad | Sentinel + failover automático | Reprogramación por `restart_policy` (caché se vacía) |
| Aislamiento de red | NetworkPolicy de K8s | overlay `internal: true` + `attachable` |
| Disrupción controlada | PodDisruptionBudget | `update_config: order: stop-first` |
| Conexión de la app | `FailoverClient` a Sentinel `:26379` | cliente directo a `valkey:6379` |

> **No aplican** aquí: Sentinel, replicación, PDB ni NetworkPolicy de K8s.

---

## 1. Prerrequisitos

### En tu laptop (controlador) — en un `.venv`, sin tocar el sistema

```bash
cd infra-valkey-swarm
python3 -m venv .venv
source .venv/bin/activate
pip install --upgrade pip
pip install "ansible-core>=2.16,<2.18"
ansible-galaxy collection install -r requirements.yml -p ./collections
```

> El SDK de Docker para Python (`python3-docker`) **no** se instala en tu laptop:
> los módulos `community.docker.*` corren en cada VPS, y `00-prereqs.yml` instala
> allí el SDK vía `apt` (servidor dedicado).

### En los VPS

- Acceso SSH como `root` (configura `inventory/hosts.yml`).
- `00-prereqs.yml` instala Docker Engine (si falta), `python3-docker`, y hace
  `docker swarm init`.

Verifica conectividad:

```bash
ansible all -i inventory/hosts.yml -m ping
```

---

## 2. Secret (ansible-vault)

```bash
openssl rand -base64 32                       # genera password
ansible-vault create group_vars/all/vault.yml # pega: vault_valkey_password: "..."
```

> Puedes usar el mismo password en todos los VPS, o uno por host con `host_vars/`.
> Cada Valkey es independiente, así que no es obligatorio compartirlo.

---

## 3. Ejecución

```bash
# 0) Docker + Swarm + SDK en cada VPS
ansible-playbook playbooks/00-prereqs.yml

# 1) Secret + stack de Valkey en cada VPS
ansible-playbook playbooks/10-valkey-deploy.yml --ask-vault-pass

# 2) Verificación
ansible-playbook playbooks/99-verify.yml --ask-vault-pass
```

Todo de golpe:

```bash
ansible-playbook -i inventory/hosts.yml \
  playbooks/00-prereqs.yml \
  playbooks/10-valkey-deploy.yml \
  playbooks/99-verify.yml \
  --ask-vault-pass
```

> Se aplica **a todos los VPS en paralelo** (cada uno su instancia local).
> Limita a uno con `--limit swarm-01`.

---

## 4. Cómo conecta tu app

La app debe correr en el mismo VPS y unirse a la red overlay `valkey_net`
(es `attachable: true`). Entonces resuelve el servicio por DNS:

```
valkey:6379   # password = el del secret (monta /run/secrets o pásalo por env)
```

Si despliegas la app como otro stack, declara la red como `external: true`:

```yaml
networks:
  valkey_net:
    external: true
    name: cache_valkey_net   # <stack>_<network>
```

---

## 5. Rollback / actualización de imagen

```bash
# Cambia valkey_image en group_vars y re-aplica (rolling update stop-first):
ansible-playbook playbooks/10-valkey-deploy.yml --ask-vault-pass

# Rollback manual del servicio a la versión anterior:
ssh root@<VPS> 'docker service rollback cache_valkey'
```

---

## 6. Desinstalación limpia

```bash
ssh root@<VPS> 'docker stack rm cache && docker secret rm valkey_password'
# (opcional) salir del swarm: docker swarm leave --force
```

> No hay volúmenes que limpiar: sin persistencia.

---

## 7. Troubleshooting

1. **`ImagePullBackOff`/pull falla de `bitnami/valkey`.** Catálogo "Bitnami Secure
   Images" (cambio 2025): prueba `bitnamilegacy/valkey:<tag>` en `valkey_image`,
   o fija un tag aún disponible.
2. **Rotar el password no surte efecto.** Los secrets de Swarm son **inmutables**.
   Crea uno versionado (`valkey_password_v2`), apúntalo en `valkey_secret_name` y
   re-despliega; luego borra el viejo.
3. **`Failed to import docker` en los módulos.** Falta `python3-docker` en el VPS
   o `ansible_python_interpreter` no apunta al Python que lo tiene. Re-ejecuta
   `00-prereqs.yml`.
4. **La app no resuelve `valkey`.** No está en la red overlay. Únela al stack o
   declara la red `external` (ver sección 4). Recuerda que `internal: true`
   impide salida a Internet pero **no** la comunicación entre servicios de la red.

Ver `CHECKLIST.md`.
