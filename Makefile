.PHONY: *

test:
	go test -v ./...

build:
	goreleaser release --clean --skip=publish --snapshot

release:
	goreleaser release --clean


kind-start:
	kind delete cluster --name=kube-resourceless
	kind create cluster --name=kube-resourceless
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.3/cert-manager.yaml

kind-stop:
	kind delete cluster --name=kube-resourceless

kind: build
	kubectl -n kube-resourceless delete deployment -l app=kube-resourceless --wait
	kind load docker-image ghcr.io/airfocusio/kube-resourceless:0.0.0-dev-amd64 --name kube-resourceless
	kubectl apply -k test/deploy/kubernetes
	sleep 10
	kubectl delete -k test/examples || true
	while ! kubectl apply -k test/examples; do sleep 1; done
