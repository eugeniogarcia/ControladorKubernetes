package v1

//Metadata La estructura de metadatos de un objeto servido por el API Server
type Metadata struct {
	Name      string
	Namespace string
}

//WebsiteSpec es la spec definida específicamente para nuestro objeto
type WebsiteSpec struct {
	GitRepo string
	Nombre  string
}

//Website es el esquema de un objeto servido por el API Server. Tiene dos propiedades la Spec y la Metadata
type Website struct {
	Metadata Metadata
	Spec     WebsiteSpec
}

//WebsiteWatchEvent Tipo con la estructura de un evento recibido por el API Server de Kubernetes
type WebsiteWatchEvent struct {
	Type   string
	Object Website
}
