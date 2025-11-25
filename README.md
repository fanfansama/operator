# FGTech Kubernetes Operator

Cet opérateur Go observe la ressource personnalisée `fgtech` (version, image, path) :
- crée un Pod + Service par CR et logge `ajout / modification / supprission` ;
- ajoute automatiquement une route dans un Ingress global (TLS) utilisant le `path` demandé (défaut : `metadata.name`).

## Prérequis
- Go 1.21+
- Docker (ou nerdctl compatible)
- Accès à un cluster Kubernetes (kubectl configuré)

## 0. Configurer l'ingress global
1. **Variables locales** : copiez `local.env.example` en `local.env`, adaptez les valeurs puis sourcez-le pour vos sessions locales :
   ```bash
   cp local.env.example local.env
   source local.env
   ```
2. **FQDN** : définissez la variable d’environnement `FGTECH_INGRESS_FQDN` (ex : `apps.local.fgtech`). Le manifeste `config/manager/manager.yaml` contient un exemple d’`env`; adaptez-le avant déploiement (ou injectez vos propres valeurs via `local.env`/`kubectl`).
2. **Secret TLS** : remplacez `REPLACE_ME_*` dans `config/ingress/tls-secret.yaml` par vos certificats Base64 puis appliquez-le dans le namespace `fgtech-system`.
4. (Optionnel) modifiez `FGTECH_INGRESS_TLS_SECRET` si vous utilisez un nom de secret différent.

## 1. Compiler localement
```bash
go mod tidy # à lancer une fois pour récupérer les dépendances
go build ./...
```

## 2. Déboguer en local
```bash
# Lance l'opérateur en utilisant le kubeconfig courant
GOMODCACHE=$(pwd)/.gomodcache GOCACHE=$(pwd)/.gocache \
  go run ./cmd --metrics-bind-address=:8080 --health-probe-bind-address=:8081
```

## 3. Déployer la CRD
```bash
kubectl apply -f config/crd/fgtech.yaml
```

## 4. Construire l'image Docker (locale)
```bash
export IMG=fgtech-operator:latest
GOMODCACHE=$(pwd)/.gomodcache GOCACHE=$(pwd)/.gocache go mod download
GOMODCACHE=$(pwd)/.gomodcache GOCACHE=$(pwd)/.gocache docker build -t "$IMG" .
```

## 5. Charger l'image dans un cluster local
> Adapté selon votre environnement local (Kind, Minikube ou Rancher Desktop). Aucun push registry n’est nécessaire.

### Kind
```bash
kind load docker-image "$IMG" --name <nom-du-cluster-kind>
```

### Minikube
```bash
minikube image load "$IMG"
```

### Rancher Desktop
Rancher Desktop utilise par défaut `nerdctl` et le runtime `containerd`. Après avoir construit l’image avec Docker ou `nerdctl`, chargez-la via :
```bash
nerdctl --address unix:///var/run/containerd/containerd.sock image load < fgtech-operator.tar
```
1. Sauvegardez l’image : `docker save "$IMG" > fgtech-operator.tar` (ou `nerdctl image save`).
2. Chargez-la dans containerd comme ci-dessus ; le cluster k3s interne la consommera ensuite automatiquement.

## 6. Déployer l'opérateur dans le cluster
```bash
kubectl apply -f config/rbac/rbac.yaml
kubectl apply -f config/ingress/tls-secret.yaml   # après avoir remplacé les données TLS
kubectl apply -f config/manager/manager.yaml
```

## 7. Vérifier le fonctionnement
```bash
kubectl -n fgtech-system get deploy/fgtech-operator
kubectl -n fgtech-system logs deploy/fgtech-operator
```

## 8. Exemple de CR `Fgtech`
```yaml
apiVersion: fgtech.fgtech.io/v1
kind: Fgtech
metadata:
  name: sample
spec:
  version: "1.0.0"
  image: ghcr.io/fgtech/app:1.0.0
  path: "/sample"
```
Appliquez-le avec :
```bash
kubectl apply -f sample-fgtech.yaml
```
Les logs de l'opérateur afficheront `ajout`, `modification` ou `supprission`. L’Ingress `fgtech-global-ingress` expose chaque `Fgtech` via le chemin configuré et redirige vers un service “fake backend” par défaut pour les autres chemins.
