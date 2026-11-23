package wompi

// Currency is an ISO 4217 currency code.
type Currency string

// CurrencyCOP is the Colombian peso, the only currency Wompi Colombia accepts.
const CurrencyCOP Currency = "COP"

// LegalIDType is a document type accepted by Wompi.
type LegalIDType string

// Document types.
const (
	LegalIDCC    LegalIDType = "CC"  // Cédula de ciudadanía
	LegalIDCE    LegalIDType = "CE"  // Cédula de extranjería
	LegalIDNIT   LegalIDType = "NIT" // Número de identificación tributaria
	LegalIDPP    LegalIDType = "PP"  // Pasaporte
	LegalIDTI    LegalIDType = "TI"  // Tarjeta de identidad
	LegalIDDNI   LegalIDType = "DNI"
	LegalIDRG    LegalIDType = "RG"
	LegalIDOther LegalIDType = "OTHER"
)

// PaymentMethodType identifies a Wompi payment method.
type PaymentMethodType string

// Payment method types.
const (
	PaymentMethodCard                PaymentMethodType = "CARD"
	PaymentMethodNequi               PaymentMethodType = "NEQUI"
	PaymentMethodPSE                 PaymentMethodType = "PSE"
	PaymentMethodBancolombia         PaymentMethodType = "BANCOLOMBIA"
	PaymentMethodBancolombiaTransfer PaymentMethodType = "BANCOLOMBIA_TRANSFER"
	PaymentMethodBancolombiaCollect  PaymentMethodType = "BANCOLOMBIA_COLLECT"
	PaymentMethodBancolombiaQR       PaymentMethodType = "BANCOLOMBIA_QR"
	PaymentMethodBancolombiaBNPL     PaymentMethodType = "BANCOLOMBIA_BNPL"
	PaymentMethodDaviplata           PaymentMethodType = "DAVIPLATA"
	PaymentMethodPCOL                PaymentMethodType = "PCOL"
	PaymentMethodSuPlus              PaymentMethodType = "SU_PLUS"
)

// TaxType is the type of a tax line.
type TaxType string

// Tax types.
const (
	TaxVAT         TaxType = "VAT"         // IVA
	TaxConsumption TaxType = "CONSUMPTION" // Impuesto al consumo
)

// Tax is a tax line of a transaction or payment link. Set AmountInCents for a
// fixed amount, or Percentage for open-amount payment links.
type Tax struct {
	Type          TaxType `json:"type"`
	AmountInCents int64   `json:"amount_in_cents,omitempty"`
	Percentage    int     `json:"percentage,omitempty"`
}

// CustomerData is the payer's information.
type CustomerData struct {
	FullName    string      `json:"full_name,omitempty"`
	PhoneNumber string      `json:"phone_number,omitempty"` // including country code, e.g. "573001234567"
	LegalID     string      `json:"legal_id,omitempty"`
	LegalIDType LegalIDType `json:"legal_id_type,omitempty"`
}

// ShippingAddress is the shipping address of a transaction.
type ShippingAddress struct {
	AddressLine1 string `json:"address_line_1"`
	AddressLine2 string `json:"address_line_2,omitempty"`
	Country      string `json:"country"` // ISO 3166-1 alpha-2, e.g. "CO"
	Region       string `json:"region"`
	City         string `json:"city"`
	Name         string `json:"name,omitempty"`
	PhoneNumber  string `json:"phone_number"`
	PostalCode   string `json:"postal_code,omitempty"`
}

// AmountInCents converts an amount in whole pesos to the cents Wompi expects,
// e.g. AmountInCents(50000) == 5000000.
func AmountInCents(pesos int64) int64 { return pesos * 100 }
