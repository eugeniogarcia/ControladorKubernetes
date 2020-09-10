package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"v1"
)

func main() {
	log.Println("website-controller started.")

	//De forma indefinida...
	for {
		//... consulta y solicita que se establezca una conexión permanente por la que el servidor nos informe de todos los cambios ?watch=true
		resp, err := http.Get("http://localhost:8001/apis/extensions.example.com/v1/websites?watch=true")
		if err != nil {
			panic(err)
		}
		//Cuando terminemos cerramos la conexión
		defer resp.Body.Close()

		//La respuesta es un json
		decoder := json.NewDecoder(resp.Body)
		//Procesamos de forma continuada. Iremos recibiendo por el stream diferentes objetos
		for {
			// El objeto debería tener esta estructura
			var event v1.WebsiteWatchEvent
			//Procesa otra entrada. Si es el final del stream termina de procesar respuestas
			if err := decoder.Decode(&event); err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			log.Printf("Received watch event: %s: %s: %s %s\n", event.Type, event.Object.Metadata.Name, event.Object.Spec.Nombre, event.Object.Spec.GitRepo)

			//Procesa el evento
			if event.Type == "ADDED" {
				createWebsite(event.Object)
			} else if event.Type == "DELETED" {
				deleteWebsite(event.Object)
			}
		}
	}

}

func createWebsite(website v1.Website) {
	createResource(website, "api/v1", "services", "service-template.json")
	createResource(website, "apis/apps/v1", "deployments", "deployment-template.json")
}

func deleteWebsite(website v1.Website) {
	deleteResource(website, "api/v1", "services", getName(website))
	deleteResource(website, "apis/apps/v1", "deployments", getName(website))
}

//Metodo que crea un recurso en el API Server
func createResource(webserver v1.Website, apiGroup string, kind string, filename string) {
	log.Printf("Creating %s with name %s in namespace %s", kind, getName(webserver), webserver.Metadata.Namespace)
	//Lee la plantilla
	templateBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	//Reemplaza de la plantilla los placeholders NAME y GIT-REPO
	template := strings.Replace(string(templateBytes), "[NAME]", getName(webserver), -1)
	template = strings.Replace(template, "[GIT-REPO]", webserver.Spec.GitRepo, -1)
	template = strings.Replace(template, "[NOMBRE]", webserver.Spec.Nombre, -1)

	//Hace el post al API Server solicitando la creación. En el body se envia el resultado de aplicar el template
	resp, err := http.Post(fmt.Sprintf("http://localhost:8001/%s/namespaces/%s/%s/", apiGroup, webserver.Metadata.Namespace, kind), "application/json", strings.NewReader(template))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("response Status:", resp.Status)
}

func deleteResource(webserver v1.Website, apiGroup string, kind string, name string) {
	log.Printf("Deleting %s with name %s in namespace %s", kind, name, webserver.Metadata.Namespace)
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:8001/%s/namespaces/%s/%s/%s", apiGroup, webserver.Metadata.Namespace, kind, name), nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("response Status:", resp.Status)

}

func getName(website v1.Website) string {
	return website.Metadata.Name + "-website"
}
