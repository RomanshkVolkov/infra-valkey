# CHECKLIST — Valkey en Docker Swarm vía Ansible

## Fase 0 — Pre-deploy
- [ ] `.venv` activado; `ansible-core` + collection `community.docker` instalados
- [ ] Acceso SSH a cada VPS verificado (`ansible all -m ping`)
- [ ] `swarm_advertise_addr` correcto por host en el inventario
- [ ] Vault creado (`vault_valkey_password`)
- [ ] `valkey_image` con tag fijo y verificado

## Fase 1 — Prerrequisitos (`00-prereqs.yml`)
- [ ] Docker Engine instalado y activo en cada VPS
- [ ] `python3-docker` presente (SDK para community.docker)
- [ ] Swarm inicializado en cada VPS (`docker info | grep Swarm: active`)

## Fase 2 — Deploy (`10-valkey-deploy.yml`)
- [ ] Docker secret `valkey_password` creado en cada VPS
- [ ] Stack `cache` desplegado; servicio `cache_valkey` con réplica 1/1
- [ ] Red overlay `valkey_net` creada (`internal`, `attachable`)

## Fase 3 — Verificación (`99-verify.yml`)
- [ ] Servicio con 1 réplica deseada
- [ ] Contenedor de la tarea en ejecución
- [ ] `valkey-cli ping` → **PONG**
- [ ] `config get maxmemory-policy` → **allkeys-lru**

## Fase 4 — Prueba de resiliencia (manual)
```bash
ssh root@<VPS>
CID=$(docker ps -q -f label=com.docker.swarm.service.name=cache_valkey)
docker rm -f "$CID"          # mata la tarea
docker service ps cache_valkey   # Swarm la reprograma
```
- [ ] Swarm recrea la tarea automáticamente (réplica vuelve a 1/1)
- [ ] La app se reconecta a `valkey:6379`
- [ ] (Esperado) La caché aparece vacía tras el reinicio — es caché, no datos durables

> NOTA: aquí NO hay failover de Sentinel ni promoción de réplica: es una
> instancia única. La "alta disponibilidad" se limita a que Swarm reprograme
> el contenedor. Si necesitas HA real de caché en Swarm, valorarías replicación
> con Sentinel multi-nodo (fuera del alcance de este setup standalone).
