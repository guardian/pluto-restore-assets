# Makefile for the Pluto Restore Assets project

.PHONY: build-local
build-local:
	eval $$(minikube -p minikube docker-env | source)
	docker build . -t guardianmultimedia/pluto-restore-assets:DEV
	docker build -f worker/Dockerfile -t guardianmultimedia/pluto-project-restore-worker:DEV .

.PHONY: deploy-local
deploy-local:
	kubectl config use-context minikube
	kubectl delete pod -l service=pluto-project-restore

.PHONY: deploy-latest
deploy-latest: build-local deploy-local
