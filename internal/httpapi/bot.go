package httpapi

import "strings"

// Session states stored in sesiones_chat.estado_sesion.
const (
	sessionAI       = "Atendida por IA"
	sessionWaiting  = "Esperando Humano"
	sessionHuman    = "Atendida por Humano"
	sessionClosed   = "Cerrada"
	botSenderType   = "bot"
	handoffReplyMsg = "Te derivo con uno de nuestros asesores. En breve se va a comunicar con vos por este mismo chat. ¡Gracias por tu paciencia!"
)

// handoffTriggers are normalized (lowercase, no accents) fragments that mean
// the customer wants a person, or that the topic needs one.
var handoffTriggers = []string{
	"asesor", "agente", "humano", "persona real", "operador", "vendedor",
	"hablar con", "comunicarme con", "quiero que me llamen", "llamenme", "llamame",
	"coordinar visita", "agendar visita", "quiero visitar", "quiero ver la propiedad",
	"reclamo", "queja", "denuncia", "urgente", "reservar", "sena ",
	"negociar", "contrato", "escritura", "tasacion",
}

// botDecision is what the bot does with an inbound message.
type botDecision struct {
	Reply   string
	Handoff bool
}

// decideBotReply is the rule-based intent layer of AdelAI. It answers simple
// questions and flags the conversation for a human when it detects a handoff
// intent. Replace the fallback branch with an LLM call when one is available.
func decideBotReply(text string) botDecision {
	normalized := normalizeIntentText(text)

	for _, trigger := range handoffTriggers {
		if strings.Contains(normalized, trigger) {
			return botDecision{Reply: handoffReplyMsg, Handoff: true}
		}
	}

	switch {
	case containsAny(normalized, "hola", "buenas", "buen dia", "buenos dias", "buenas tardes", "buenas noches"):
		return botDecision{Reply: "¡Hola! Soy AdelAI, el asistente de Adelson Servicios Inmobiliarios. Contame qué tipo de propiedad buscás (compra o alquiler, zona, ambientes y presupuesto) y te ayudo. Si preferís hablar con un asesor, escribí \"asesor\"."}
	case containsAny(normalized, "alquiler", "alquilar"):
		return botDecision{Reply: "Perfecto, ¿en qué zona buscás alquilar, cuántos ambientes necesitás y cuál es tu presupuesto mensual?"}
	case containsAny(normalized, "comprar", "compra", "venta", "vender"):
		return botDecision{Reply: "Genial. ¿Qué zona te interesa, cuántos ambientes necesitás y qué presupuesto manejás? Con eso te muestro opciones."}
	case containsAny(normalized, "precio", "cuanto sale", "cuanto cuesta", "valor"):
		return botDecision{Reply: "Los precios dependen de cada propiedad. Decime cuál te interesa (zona o código) y te paso el detalle. Si querés una cotización personalizada, escribí \"asesor\"."}
	case containsAny(normalized, "horario", "direccion", "ubicacion", "donde estan"):
		return botDecision{Reply: "Para horarios y ubicación de la inmobiliaria, un asesor te lo confirma. Escribí \"asesor\" y te derivo."}
	case containsAny(normalized, "gracias", "chau", "adios"):
		return botDecision{Reply: "¡De nada! Cualquier otra consulta, escribime cuando quieras."}
	}

	return botDecision{Reply: "No estoy seguro de haber entendido. ¿Podés contarme qué propiedad buscás (zona, ambientes y presupuesto)? Si preferís, escribí \"asesor\" y te derivo con una persona."}
}

func containsAny(s string, fragments ...string) bool {
	for _, f := range fragments {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}

// normalizeIntentText lowercases and strips accents so matching is tolerant of
// how customers type ("tasación", "TASACION", "tasacion").
func normalizeIntentText(s string) string {
	return accentReplacer.Replace(strings.ToLower(s))
}

var accentReplacer = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
