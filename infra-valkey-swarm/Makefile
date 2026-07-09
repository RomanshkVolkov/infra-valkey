# =====================================================================
# Makefile — atajos para desplegar Valkey (standalone) en Docker Swarm.
#
#   make setup    # .venv + Ansible + collection community.docker (una vez)
#   make config   # crea inventory/group_vars/all/config.yml desde la plantilla
#   <edita config.yml: alias SSH, imagen, memoria>
#   make deploy   # despliega TODO (prereqs + Valkey + verificación)
# =====================================================================

VENV    := .venv
PY      := python3
PIP     := $(VENV)/bin/pip
ANSIBLE := $(VENV)/bin/ansible-playbook
GALAXY  := $(VENV)/bin/ansible-galaxy
CONFIG  := inventory/group_vars/all/config.yml

.DEFAULT_GOAL := help

.PHONY: help setup config deploy verify clean reset

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

setup: ## Crea el .venv e instala Ansible y la collection community.docker (una vez)
	$(PY) -m venv $(VENV)
	$(PIP) install --upgrade pip
	$(PIP) install "ansible-core>=2.16,<2.18"
	$(GALAXY) collection install -r requirements.yml -p ./collections
	@echo ">> Listo. Ahora: make config"

config: ## Crea inventory/group_vars/all/config.yml desde la plantilla (si no existe)
	@if [ -f $(CONFIG) ]; then \
		echo ">> $(CONFIG) ya existe (no se sobrescribe)."; \
	else \
		mkdir -p $(dir $(CONFIG)); \
		cp config.example.yml $(CONFIG); \
		echo ">> Creado $(CONFIG). EDÍTALO (alias SSH, imagen) y luego: make deploy"; \
	fi

deploy: $(CONFIG) ## Despliega todo: Docker/Swarm + Valkey + verificación
	$(ANSIBLE) site.yml

verify: $(CONFIG) ## Solo re-ejecuta la verificación post-deploy
	$(ANSIBLE) playbooks/99-verify.yml

clean: $(CONFIG) ## Desinstala el stack de Valkey y su secret del VPS
	$(ANSIBLE) playbooks/99-cleanup.yml

reset: ## Borra el .venv y las collections locales (no toca el VPS)
	rm -rf $(VENV) collections

$(CONFIG):
	@echo "ERROR: falta $(CONFIG). Ejecuta primero 'make config' y edítalo." >&2
	@exit 1
