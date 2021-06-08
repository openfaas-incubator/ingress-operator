IngressOperator for OpenFaaS
====

Get custom domains and TLS for your OpenFaaS Functions through the FunctionIngress CRD

[![Build Status](https://travis-ci.com/openfaas-incubator/ingress-operator.svg?branch=master)](https://travis-ci.com/openfaas-incubator/ingress-operator)
[![OpenFaaS](https://img.shields.io/badge/openfaas-serverless-blue.svg)](https://www.openfaas.com)

## Why is this needed?

OpenFaaS functions are created as pairs with Deployment and Service objects, which eventually go on to create a Pod.

Deployments should not be exposed directly, but accessed via the OpenFaaS Gateway service.

The gateway in OpenFaaS has a number of roles including:
* providing HA through N replicas
* adding tracing IDs
* adding authz
* collecting metrics
* scaling endpoints from zero

Users started to create `Ingress` records pointing at the gateway for each public endpoint they wanted to host with a specific website address. This Operator automates that.

This project addresses the following Proposal for Kubernetes: [Proposal: define custom hostname for functions #1082](https://github.com/openfaas/faas/issues/1082)

> Looking for a tutorial? See the [OpenFaaS documentation](https://docs.openfaas.com/reference/authentication/)

Minimum supported cert-manager version: [v1.0](https://cert-manager.io/docs/release-notes/release-notes-1.0/)

## Schema

This is an Operator / controller to build Kubernetes `Ingress` and `cert-manager` `Certificate` objects for functions.

The following example would expose the `nodeinfo` function from the store as a URL: `nodeinfo.myfaas.club`.

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo
  namespace: openfaas
spec:
  domain: "nodeinfo.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
  path: "/v1/profiles/(.*)" # Optionally set a path for the domain i.e. nodeinfo.myfaas.club/v1/profiles/
  # tls:
  #   enabled: true
  #   issuerRef:
  #     name: "letsencrypt-staging"
  #     kind: "Issuer"
```

Exploring the schema:

* The `domain` field corresponds to a DNS entry which points at your IngressController's public IP, or the IP of one of the hosts if using `HostPort`.
* `function` refers to the function you want to expose on the domain.
* `path` set a root path / prefix for the function to be mounted at the domain specified in `domain`
* `tls` whether to provision a TLS certificate using JetStack's [cert-manager](https://github.com/jetstack/cert-manager)
* `issuerRef` which issuer to use, this may be a staging or production issuer.
* `issuerRef.kind` Issuer or ClusterIssuer, This depends on whats available in your cluster

### REST-style mapping of functions

See an example in the [OpenFaaS docs](https://docs.openfaas.com/reference/ssl/kubernetes-with-cert-manager/#30-rest-style-api-mapping-for-your-functions)

## Status

Completed backlog items:

- [x] Create `Ingress` records for HTTP
- [x] Create `Ingress` records for HTTPS
- [x] Create cert-manager `Certificate` records
- [x] Support Nginx
- [x] Support Zoolando's Skipper
- [x] Support Traefik
- [x] Support armhf / Raspberry Pi
- [x] Add `.travis.yml` for CI
- [x] REST-style path prefixes for functions

Remaining items:

- [ ] Synchronise annotations upon edit of FunctionIngress CRs [#39](https://github.com/openfaas/ingress-operator/issues/)

## Deployment

### Pre-reqs

There are several pre-reqs for a working installation, but some of these components are installed with OpenFaaS and can also be found in [the docs](https://docs.openfaas.com/).

#### IngressController: `nginx`

[nginx IngressController](https://github.com/helm/charts/tree/master/stable/nginx-ingress) is recommended. Use a HostPort if testing against a local cluster where `LoadBalancer` is unavailable.

Make sure you have [helm](https://github.com/openfaas/faas-netes/blob/master/HELM.md) and Tiller.

Install [nginx](https://nginx.org/en/docs/) with LoadBalancer:

```sh
helm install stable/nginx-ingress --name nginxingress --set rbac.create=true
```

Install nginx with host-port:

```sh
export ADDITIONAL_SET=",controller.hostNetwork=true,controller.daemonset.useHostPort=true,dnsPolicy=ClusterFirstWithHostNet,controller.kind=DaemonSet"
helm install stable/nginx-ingress --name nginxingress --set rbac.create=true${ADDITIONAL_SET}
```

#### OpenFaaS

OpenFaaS is also required:

```
git clone https://github.com/openfaas/faas-netes
cd faas-netes

kubectl apply -f namespaces.yml
# generate a random password
PASSWORD=$(head -c 12 /dev/urandom | shasum| cut -d' ' -f1)

kubectl -n openfaas create secret generic basic-auth \
--from-literal=basic-auth-user=admin \
--from-literal=basic-auth-password="$PASSWORD"

echo $PASSWORD > ../password.txt

kubectl apply -f ./yaml

kubectl port-forward -n openfaas deploy/gateway 31112:8080 &
echo -n ${PASSWORD} | faas-cli login --username admin --password-stdin -g 127.0.0.1:31112
faas-cli store deploy nodeinfo -g 127.0.0.1:31112
```

#### Configure DNS records

##### Find your public IP

Find the LB for Nginx:

```
kubectl get svc -n default
```

Or find the NodeIP:

```
kubectl get node -o wide
```

##### Create DNS A records

You should now configure your DNS A records:

For example, `nodeinfo` function in the `myfaas.club` domain and IP `178.128.137.209`:

```
nodeinfo.myfaas.club  178.128.137.209
```

> Note: with DigitalOcean's CLI you could run: `doctl compute domain create nodeinfo.myfaas.club --ip-address 178.128.137.209`.

##### TLS: Configure cert-manager

If using TLS, then install [cert-manager](https://docs.openfaas.com/reference/ssl/kubernetes-with-cert-manager/#install-cert-manager).

Now [create an issuer](https://cert-manager.io/docs/configuration/) to use the staging endpoint:

```yaml
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt-staging
  namespace: openfaas
spec:
  acme:
    # The ACME server URL
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    # Email address used for ACME registration
    email: user@example.com
    # Name of a secret used to store the ACME account private key
    privateKeySecretRef:
      name: letsencrypt-staging
    # Enable the HTTP-01 challenge provider
    solvers:
    # An empty 'selector' means that this solver matches all domains
    - selector: {}
      http01:
        ingress:
          class: nginx
```

or ClusterIssuer

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
  namespace: openfaas
spec:
  acme:
    # The ACME server URL
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    # Email address used for ACME registration
    email: user@example.com
    # Name of a secret used to store the ACME account private key
    privateKeySecretRef:
      name: letsencrypt-staging
    # Enable the HTTP-01 challenge provider
    solvers:
    # An empty 'selector' means that this solver matches all domains
    - selector: {}
      http01:
        ingress:
          class: nginx
```

* Edit the `email` and take note of the `namespace`, you will want this to be `openfaas`.
* If using `traefik` instead of `nginx`, then edit `class: nginx` and replace it as necessary.
  **Recommended version is v1.7.21 or above**, previous versions will incorrectly route requests
  to your function (with duplicated path, see
  [related issue](https://github.com/openfaas-incubator/ingress-operator/issues/30)).

Save as `letsencrypt-issuer.yaml` then run `kubectl apply -f letsencrypt-issuer.yaml`.

If you are confident in the configuration, switch over to the production issuer, but note that it is rate-limited.

* Change `letsencrypt-staging` to `letsencrypt-prod`
* Edit `https://acme-staging-v02.api.letsencrypt.org/directory` to `https://acme-v02.api.letsencrypt.org/directory`

Save the file and apply.

### Custom annotations

You can also set custom annotations to be passed down to the Ingress record created by the operator.

Example:

This example adds one of the required annotations for basic auth as defined in the [ingress-nginx docs](https://kubernetes.github.io/ingress-nginx/examples/auth/basic/).

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo
  namespace: openfaas
  annotations:
    nginx.ingress.kubernetes.io/auth-type: basic
spec:
  domain: "nodeinfo.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
```

### Asynchronous functions

This example exposes the nodeinfo function for asynchronous invocation by rewriting its path to the gateway URL including the `/async-function` prefix instead of the usual `/function/`.

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo
  namespace: openfaas
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /async-function/nodeinfo/$1
spec:
  domain: "nodeinfo.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
  ```

### Bypass mode

The IngressOperator can be used to create Ingress records that bypass the OpenFaaS Gateway. This may be useful when you are running a non-standard workload such as a brownfields monolith to reduce hops, or with an unsupported protocol like gRPC or websockets.

Example:

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo
  namespace: openfaas-fn
spec:
  domain: "nodeinfo.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
  bypassGateway: true
```

Note that since Ingress records must be created in the same namespace as the backend service, `namespace` is changed to `openfaas-fn`.

By default, the OpenFaaS helm chart can deploy the first instance of the operator, if you need gateway bypass, then deploy a second operator using a customised version of `artifacts/operator-amd64.yaml`.

When deploying the operator, you will also need to:

* Set the `ingress_namespace` env-var to `openfaas-fn`
* Edit the deployment `namespace` to `openfaas-fn`
* Optionally: edit `artifacts/operator-rbac.yaml` to `openfaas-fn` and apply

### Run or deploy the IngressOperator

#### In-cluster:

```sh
kubectl apply -R -f ./artifacts/

kubectl logs -n openfaas deploy/ingress-operator
```

#### Local testing:

```sh
rm ./artifacts/operator-amd64.yaml
kubectl apply -R -f ./artifacts/

go build && ./ingress-operator -kubeconfig=./config
```

## Create your own `FunctionIngress`

### With TLS

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo-tls
  namespace: openfaas
spec:
  domain: "nodeinfo-tls.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
  tls:
    enabled: true
    issuerRef:
      name: "letsencrypt-staging"
      # Change to ClusterIssuer if required
      # https://cert-manager.io/docs/concepts/issuer/
      kind: "Issuer"
```

*nodeinfo.yaml*

### Without TLS

```yaml
apiVersion: openfaas.com/v1alpha2
kind: FunctionIngress
metadata:
  name: nodeinfo
  namespace: openfaas
spec:
  domain: "nodeinfo.myfaas.club"
  function: "nodeinfo"
  ingressType: "nginx"
```

*nodeinfo.yaml*

### Apply

```sh
kubectl apply -f nodeinfo.yaml
```

### Test:

```
# Find the ingress record
kubectl get ingress -n openfaas

# Find the cert record
kubectl get cert -n openfaas

# Find the FunctionIngress
kubectl get FunctionIngress -n openfaas
```

Remember to configure DNS for `nodeinfo.myfaas.club` or edit `/etc/hosts` and point to your `IngressController`'s IP or `LoadBalancer`.

## Kubernetes versions
Ingress Operator currently requires Kubernetes version 1.16+

## Contributing

This project follows the [OpenFaaS contributing guide](./CONTRIBUTING.md)

## Configuration via Environment Variable

| Option              | Usage                                                                                              |
|---------------------|----------------------------------------------------------------------------------------------------|
| `ingress_namespace` | Namespace to create Ingress within, if bypassing gateway, set to `openfaas-fn`. default: `openfaas`|

## LICENSE

MIT
