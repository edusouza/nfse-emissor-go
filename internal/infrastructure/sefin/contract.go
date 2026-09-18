package sefin

// The constants below are taken from the government's own OpenAPI documents,
// both versioned under docs/api/:
//
//   - sefin-nacional-swagger.json ("API NFS-e - Sefin Nacional") — emission,
//     lookup and events. This is the contract this client implements.
//   - adn-contribuinte-swagger.json ("API NFS-e - ADN Contribuinte") —
//     document distribution by NSU. Not used yet.
//
// An earlier version of this client invented a SOAP envelope for a REST API,
// which is why nothing it produced ever reached the government. Everything here
// is now traceable to a published specification; when something is not, say so
// in a comment rather than letting it look settled.

// Base URLs of the Sefin Nacional service.
//
// The path segment is spelled "SefinNacional" in the specification's basePath.
// It is capitalised, and path matching is case-sensitive.
const (
	// ProductionBaseURL issues invoices with fiscal value.
	ProductionBaseURL = "https://sefin.nfse.gov.br/SefinNacional"

	// RestrictedProductionBaseURL is the government's testing environment,
	// declared as the specification's host. Invoices issued here carry no
	// fiscal value.
	RestrictedProductionBaseURL = "https://sefin.producaorestrita.nfse.gov.br/SefinNacional"
)

// Environment codes used by tipoAmbiente throughout the API.
//
// Note the naming: the government calls environment 2 "Homologação" in the
// schema descriptions while hosting it at producaorestrita. They are the same
// thing.
const (
	// EnvCodeProduction is tipoAmbiente 1 — invoices have fiscal value.
	EnvCodeProduction = 1

	// EnvCodeRestrictedProduction is tipoAmbiente 2 — invoices do not.
	EnvCodeRestrictedProduction = 2
)

// Request fields carrying a gzipped, base64-encoded XML document.
const (
	// fieldSignedDPS is the body property of POST /nfse.
	fieldSignedDPS = "dpsXmlGZipB64"

	// fieldEventRequest is the body property of POST /nfse/{chave}/eventos.
	fieldEventRequest = "pedidoRegistroEventoXmlGZipB64"
)

// fieldAuthorizedNFSe is the response property holding the issued NFS-e, named
// here only so that a diagnostic can point at it.
const fieldAuthorizedNFSe = "nfseXmlGZipB64"

// Endpoint paths, relative to the base URL.
//
// Two services that used to live here have moved to the ADN, and the
// specification answers 501 for them:
//
//	GET /DANFSe             -> adn.../danfse/docs
//	GET /ParametrosMunicipais -> adn.../parametrizacao/docs
const (
	pathEmit   = "/nfse"
	pathNFSe   = "/nfse/"
	pathDPS    = "/dps/"
	pathEvents = "/eventos"
)
