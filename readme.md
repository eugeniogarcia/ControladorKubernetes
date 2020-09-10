# Create the Custom Resource Definition (CRD)

We create a CRD for `websites`:

```ps
kubectl apply -f website-crd.yaml
```

Now we can create an object of this type:

```ps
kubectl create -f kubia-website.yaml
```

We can see the object created:

```ps
kubectl get websites

NAME    AGE
kubia   3s
```

We will delete the object now:

```ps
kubectl delete website kubia
```

# Sidecar

Nuestro controller necesita interactura con el API Server. Para simplificar estas llamadas vamos a crear un container que actuara de proxy, y que se ejecutara como sidecar. Creamos la imagen:

```ps
docker build -t egsmartin/kubectl-proxy .
```

La publicamos al registry:

```ps
docker push egsmartin/kubectl-proxy:latest

The push refers to repository [docker.io/egsmartin/kubectl-proxy]
59c966300e31: Pushed
e09dc50fc029: Pushing  1.649MB/55.36MB
50644c29ef5a: Mounted from library/alpine
```

# Build the Controller container

First we set-up the configuration for go. We configure the `GOPATH`, and specify that we want to compile the controller for linux:

```ps
$a=pwd

$Env:GOPATH=$Env:GOPATH+$a.Path

$Env:GOOS="linux"
$Env:CGO_ENABLED=0

go build -o website-controller -a website-controller.go
```

We can now create the container for our controller:

```ps
docker build -t egsmartin/website-controller .
```

```ps
docker images

REPOSITORY                     TAG                 IMAGE ID            CREATED             SIZE
egsmartin/website-controller   latest              a5b1b83bdda4        17 seconds ago      7.3MB
```

We now push the image to our registry:

```ps
docker push egsmartin/website-controller:latest

The push refers to repository [docker.io/egsmartin/website-controller]
c8fdde4564b7: Pushed
175e20412fb5: Pushed
f030ddd82994: Pushing  1.868MB/7.3MB
```

# Ejecutar el controller

El controller se ejecutara como un pod más en el cluster. En primer lugar necesitamos crear una service account para ejecutar nuestro pod:

```ps
kubectl create serviceaccount website-controller
```

```ps
kubectl create clusterrolebinding website-controller --clusterrole=cluster-admin --serviceaccount=default:website-controller

clusterrolebinding.rbac.authorization.k8s.io/website-controller created
```

Finalmente creamos el pod. Hemos creado un deployment con la imagen del controller y con el sidecar que hace de proxy con el servidor de APIs:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: website-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: website-controller
  template:
    metadata:
      name: website-controller
      labels:
        app: website-controller
    spec:
      serviceAccountName: website-controller
      containers:
      - name: main
        image: egsmartin/website-controller
      - name: proxy
        image: egsmartin/kubectl-proxy:latest
```

```ps
kubectl apply -f .\website-controller.yaml
```

# Analisis

El CRD que hemos creado:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: websites.extensions.example.com
spec:
  scope: Namespaced
  group: extensions.example.com
  version: v1
  names:
    kind: Website
    singular: website
    plural: websites
```

Es `namespaced`, se llama `website`, con un grupo `extensions.example.com` y la versión es `v1`. Cuando queramos interrogar al servidor de Kubernetes el recurso será `http://localhost:8001/apis/extensions.example.com/v1/websites`.

Cuando nuestro controller arranque se conectara con el API Server y comenzara a monitorizar cambios:

```go
resp, err := http.Get("http://localhost:8001/apis/extensions.example.com/v1/websites?watch=true")
```

El API server enviara watch events con cada cambio en el objeto Website. El API server envia el `ADDED` watch event cada vez que un objeto Website es creado. 

Cuando recibimos un evento extraemos las propiedades del evento:

```go
log.Printf("Received watch event: %s: %s: %s\n", event.Type, event.Object.Metadata.Name, event.Object.Spec.GitRepo)
```

y lo procesa:

```go
if event.Type == "ADDED" {
    createWebsite(event.Object)
} else if event.Type == "DELETED" {
    deleteWebsite(event.Object)
}
```

## Crea Recurso

Hace un POST al API server con solicitando la creación del recurso:

```go
resp, err := http.Post(fmt.Sprintf("http://localhost:8001/%s/namespaces/%s/%s/", apiGroup, webserver.Metadata.Namespace, kind), "application/json", strings.NewReader(template))
```

El payload se construye con un template que leemos de disco, y en el que reemplazamos una serie de placeholders:

```go
templateBytes, err := ioutil.ReadFile(filename)
if err != nil {
    log.Fatal(err)
}
//Reemplaza de la plantilla los placeholders NAME y GIT-REPO
template := strings.Replace(string(templateBytes), "[NAME]", getName(webserver), -1)
template = strings.Replace(template, "[GIT-REPO]", webserver.Spec.GitRepo, -1)
```

## Borra Recurso

Hace una peticion DELETE al API server con el objeto que debe ser eliminado:

```go
req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:8001/%s/namespaces/%s/%s/%s", apiGroup, webserver.Metadata.Namespace, kind, name), nil)
```

## ADDED

Con este evento creamos dos recursos, un deployment y un servicio:

```go
createResource(website, "api/v1", "services", "service-template.json")
createResource(website, "apis/extensions/v1beta1", "deployments", "deployment-template.json")
```

## DELETED

Con este evento borramos los dos recursos que creamos al crear el objeto:

```go
deleteResource(website, "api/v1", "services", getName(website))
deleteResource(website, "apis/extensions/v1beta1", "deployments", getName(website))
```

## Estructura del CRD
Aqui hemos usado la especificación `apiextensions.k8s.io/v1beta1` para definir un CRD. En esta especificación de la api es opcional dar una estructura al objeto. Con `apiextensions.k8s.io/v1` es obligatorio definir una estructura en el `CustomResourceDefinitions`.

Los CustomResources almacenan los datos en campos custom (junto con los campos built-in: apiVersion, kind y metadata, que son validados por el API server de forma implicita). Para definir la estructura podemos usar `OpenAPI v3.0` de forma que al crear una instancia de un determinado objeto, Kubernetes valide que la estrucutura sea correcta.