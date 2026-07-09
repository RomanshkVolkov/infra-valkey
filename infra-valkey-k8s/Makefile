# =====================================================================
# Makefile — atajos para desplegar Valkey en k3s con un solo comando.
#
# Flujo típico desde cero:
#   make setup     # crea el .venv e instala Ansible + collection (una vez)
#   make config    # crea group_vars/all/config.yml a partir de la plantilla
#   <edita group_vars/all/config.yml: IP, clave SSH, imagen de la app>
#   make deploy    # despliega TODO (kubeconfig + Valkey + app + verificación)
# =====================================================================

VENV    := .venv
PY      := python3
PIP     := $(VENV)/bin/pip
GALAXY  := $(VENV)/bin/ansible-galaxy
CONFIG  := inventory/group_vars/all/config.yml
# Lanzamos ansible-playbook con .venv/bin en el PATH para que los módulos de
# helm encuentren el binario `helm` instalado ahí por `make setup`.
ANSIBLE := PATH="$(CURDIR)/$(VENV)/bin:$$PATH" $(VENV)/bin/ansible-playbook

.DEFAULT_GOAL := help

.PHONY: help setup config deploy verify clean reset

help: ## Muestra esta ayuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

setup: ## Crea el .venv e instala Ansible, libs Python, la collection y helm (una vez)
	$(PY) -m venv $(VENV)
	$(PIP) install --upgrade pip
	$(PIP) install "ansible-core>=2.16,<2.18" kubernetes kubernetes-validate pyyaml
	$(GALAXY) collection install -r requirements.yml -p ./collections
	@if [ -x $(VENV)/bin/helm ]; then \
		echo ">> helm ya presente en $(VENV)/bin"; \
	else \
		echo ">> instalando helm en $(VENV)/bin (sin sudo)"; \
		curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \
			| HELM_INSTALL_DIR=$(CURDIR)/$(VENV)/bin PATH="$(CURDIR)/$(VENV)/bin:$$PATH" bash -s -- --no-sudo; \
	fi
	@echo ">> Listo. Ahora: make config"

config: ## Crea inventory/group_vars/all/config.yml desde la plantilla (si no existe)
	@if [ -f $(CONFIG) ]; then \
		echo ">> $(CONFIG) ya existe (no se sobrescribe)."; \
	else \
		mkdir -p $(dir $(CONFIG)); \
		cp config.example.yml $(CONFIG); \
		echo ">> Creado $(CONFIG). EDÍTALO (IP, clave SSH, imagen) y luego: make deploy"; \
	fi

deploy: $(CONFIG) ## Despliega todo: kubeconfig + Valkey + app + verificación
	$(ANSIBLE) site.yml

verify: $(CONFIG) ## Solo re-ejecuta la verificación post-deploy
	$(ANSIBLE) playbooks/99-verify.yml

clean: $(CONFIG) ## Desinstala Valkey, la app y los namespaces del clúster
	$(ANSIBLE) playbooks/99-cleanup.yml

reset: ## Borra el .venv y las collections locales (no toca el clúster)
	rm -rf $(VENV) collections

# Falla con un mensaje claro si falta la config en vez de un error críptico.
$(CONFIG):
	@echo "ERROR: falta $(CONFIG). Ejecuta primero 'make config' y edítalo." >&2
	@exit 1
