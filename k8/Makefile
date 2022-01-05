include ../.bingo/Variables.mk

.DEFAULT_GOAL=deploy

deploy: $(KUSTOMIZE)
	cd ${DEPLOYMENT} && $(KUSTOMIZE) edit set image tableland/api=${IMAGE}
	cd ${DEPLOYMENT} && $(KUSTOMIZE) build . | kubectl apply -f -
.PHONY: deploy
