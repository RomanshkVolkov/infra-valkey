# infra-valkey

Automatización con Ansible para desplegar [Valkey](https://valkey.io/) (cache
tipo Redis) en alta disponibilidad, con dos topologías independientes:

- **[`infra-valkey-k8s/`](infra-valkey-k8s/)** — Valkey en Kubernetes (k3s) vía
  el chart de Bitnami, en modo Sentinel (1 primary + 2 réplicas + 3 sentinels),
  con NetworkPolicy y una app de ejemplo (cache-aside en Go) opcional.
- **[`infra-valkey-swarm/`](infra-valkey-swarm/)** — Valkey en Docker Swarm
  sobre un VPS, con el password gestionado como Docker secret.

Cada carpeta es autocontenida: mira su `README.md` y `CHECKLIST.md`.

## Uso rápido

```bash
cd infra-valkey-k8s      # o infra-valkey-swarm
make config              # copia config.example.yml -> group_vars/all/config.yml
$EDITOR inventory/group_vars/all/config.yml
make deploy
```

## Seguridad

La config real y los secretos **nunca** se versionan (ver `.gitignore` de cada
carpeta): `group_vars/all/config.yml`, `vault.yml`, `.valkey_password`, el
`.venv/` y las `collections/` instaladas quedan fuera del repo. Lo versionado
son solo plantillas (`*.example.yml`, `vault.yml.example`) con placeholders.
