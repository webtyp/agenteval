package ui

// The words the person sees. Spanish: the lab is used by the clinic's developers.
const (
	Title         = "Laboratorio de agentes"
	WakeLabel     = "Despertar al agente"
	WakeHint      = "La primera vez descarga sus modelos (≈ 1,2 GB) y los guarda en este equipo."
	Downloading   = "Descargando %s: %d %%" // artifact id, percent
	Loading       = "Cargando los modelos…"
	ReadyNote     = "Listo."
	WriterOffNote = "Este equipo no alcanza para el redactor: responde con datos y plantillas."
	NotPersisted  = "Este navegador podría borrar los modelos si se llena el disco."
	AsleepTitle   = "El agente no puede funcionar en este equipo."
	Placeholder   = "Escribe tu pregunta…"
	SendLabel     = "Enviar"
	ConfirmLabel  = "Confirmar"
	DeclineLabel  = "Cancelar"
	FailedPrefix  = "Error: "
	Me            = "Tú"
	Agent         = "Agente"
	SessionID     = "lab"
	MaxBodyLength = 2000
)

// shortfallTexts says each device.Shortfall in Spanish (a slice, not a map: TinyGo).
var shortfallTexts = []struct{ name, text string }{
	{"secure", "La página no está en HTTPS."},
	{"space", "No hay espacio suficiente en este equipo."},
	{"tier", "Este navegador no puede ejecutar el modelo."},
	{"speed", "Este equipo es demasiado lento para el agente."},
	{"memory", "Este equipo tiene poca memoria."},
}

// ShortfallText says a shortfall's name (device.Shortfall.String()) in Spanish; an unknown
// name is returned as is.
func ShortfallText(name string) string {
	for _, s := range shortfallTexts {
		if s.name == name {
			return s.text
		}
	}
	return name
}
