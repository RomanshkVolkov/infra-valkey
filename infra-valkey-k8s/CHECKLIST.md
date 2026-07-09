# CHECKLIST — despliegue Valkey en K8s vía Ansible

Marca cada ítem. La fase "Post-deploy" la automatiza `playbooks/99-verify.yml`.

## Fase 0 — Pre-deploy

- [ ] `make setup` ejecutado (.venv + Ansible + libs + collection `kubernetes.core`)
- [ ] `make config` ejecutado y `group_vars/all/config.yml` editado
- [ ] `node_ip` y `ssh_key` rellenados; acceso SSH al nodo OK
- [ ] `valkey_chart_version` verificada (`helm search repo bitnami/valkey --versions`)
- [ ] `valkey_password` vacío (autogenera) o con tu propio valor
- [ ] (El kubeconfig lo trae `make deploy` automáticamente — no hay paso manual)
- [ ] (Solo si `deploy_app_ejemplo: true`) `app_image` apunta a una imagen publicada y fijada por tag/digest

## Fase 1 — Prerrequisitos (`00-prereqs.yml`)

- [ ] Helm 3 disponible en el nodo (`helm version --short`)
- [ ] Repo Helm `bitnami` registrado

## Fase 2 — Post-deploy de Valkey (`10-valkey-deploy.yml` + `99-verify.yml`)

- [ ] Namespaces `data` y `production` creados (con label `name=...`)
- [ ] Secret `valkey-auth` presente en `data`
- [ ] **3 pods de Valkey** en `Running` y `Ready`
- [ ] **3 Sentinels** arriba (contenedor `valkey-sentinel` Ready en cada pod)
- [ ] `valkey-cli ping` responde **PONG**
- [ ] `info replication` muestra `role:master` y `connected_slaves:2`
- [ ] `sentinel master mymaster` muestra `num-slaves=2` y `num-other-sentinels=2`
- [ ] NetworkPolicy aplicada (recuerda: requiere CNI compatible, ver troubleshooting)

## Fase 3 — Post-deploy de la app (OPCIONAL: solo con `deploy_app_ejemplo: true`)

- [ ] Secret `valkey-auth` replicado a `production`
- [ ] Deployment `app-ejemplo` con réplicas disponibles
- [ ] App responde **HTTP 200** en `/usuario/1`
- [ ] Cache **MISS** (~50ms) en la 1ª petición a un ID nuevo
- [ ] Cache **HIT** (<5–15ms) en la 2ª petición al mismo ID

## Fase 4 — Prueba de failover (manual)

```bash
KUBECONFIG=~/.kube/k3s-valkey.yaml

# 1) Identifica el pod master actual
kubectl -n data exec <pod-valkey> -c valkey -- \
  sh -c 'valkey-cli -a "$VALKEY_PASSWORD" --no-auth-warning info replication' | grep role

# 2) Borra el pod master
kubectl -n data delete pod <pod-master>
```

- [ ] Sentinel promueve una réplica a master en **< 30s** (`down-after-ms=60000` da margen)
- [ ] `sentinel master mymaster` apunta al nuevo master
- [ ] La app sigue respondiendo (el FailoverClient redescubre el master vía Sentinel)
- [ ] El pod borrado vuelve como réplica del nuevo master
