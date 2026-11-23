package wompi

// PaymentMethod is the payment_method object sent when creating a
// transaction. Prefer the constructors below (CardPayment, NequiPayment,
// PSEPayment, ...), which set exactly the fields each method requires.
type PaymentMethod struct {
	// Type is required for payer-present payments. It is omitted for card
	// payment source charges (see SourceInstallments).
	Type PaymentMethodType `json:"type,omitempty"`

	// CARD
	Token        string `json:"token,omitempty"`
	Installments int    `json:"installments,omitempty"`

	// NEQUI, BANCOLOMBIA_BNPL
	PhoneNumber string `json:"phone_number,omitempty"`
	PhoneCode   string `json:"phone_code,omitempty"`

	// PSE (int 0/1, see PSEUserType) and BANCOLOMBIA_TRANSFER ("PERSON").
	UserType any `json:"user_type,omitempty"`

	// PSE, DAVIPLATA, SU_PLUS, BANCOLOMBIA_BNPL
	UserLegalIDType LegalIDType `json:"user_legal_id_type,omitempty"`
	UserLegalID     string      `json:"user_legal_id,omitempty"`

	// PSE
	FinancialInstitutionCode string `json:"financial_institution_code,omitempty"`
	ReferenceOne             string `json:"reference_one,omitempty"`
	ReferenceTwo             string `json:"reference_two,omitempty"`
	ReferenceThree           string `json:"reference_three,omitempty"`

	// PSE, BANCOLOMBIA_*, DAVIPLATA, BANCOLOMBIA_BNPL
	PaymentDescription string `json:"payment_description,omitempty"`

	// BANCOLOMBIA_TRANSFER
	EcommerceURL string `json:"ecommerce_url,omitempty"`

	// BANCOLOMBIA_BNPL
	Name        string `json:"name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	RedirectURL string `json:"redirect_url,omitempty"`

	// SandboxStatus forces the final status in sandbox for BANCOLOMBIA_QR,
	// BANCOLOMBIA_COLLECT (where sandbox requires it), BANCOLOMBIA_TRANSFER
	// and PCOL. Ignored in production.
	SandboxStatus string `json:"sandbox_status,omitempty"`
}

// PSEUserType is the PSE payer type.
type PSEUserType int

// PSE payer types.
const (
	PSEUserNatural PSEUserType = 0 // persona natural
	PSEUserLegal   PSEUserType = 1 // persona jurídica
)

// CardPayment charges a card token (from Tokens.CreateCard or the Wompi
// widget) in the given number of installments (1 for a single payment).
func CardPayment(token string, installments int) *PaymentMethod {
	if installments < 1 {
		installments = 1
	}
	return &PaymentMethod{Type: PaymentMethodCard, Token: token, Installments: installments}
}

// SourceInstallments is the payment method of a charge to a CARD payment
// source: Wompi requires the number of installments alongside
// CreateTransactionParams.PaymentSourceID.
func SourceInstallments(installments int) *PaymentMethod {
	if installments < 1 {
		installments = 1
	}
	return &PaymentMethod{Installments: installments}
}

// NequiPayment sends a push notification to a 10-digit Nequi phone number.
// The result arrives asynchronously (event or polling).
func NequiPayment(phoneNumber string) *PaymentMethod {
	return &PaymentMethod{Type: PaymentMethodNequi, PhoneNumber: phoneNumber}
}

// PSEPaymentParams are the fields of a PSE payment.
type PSEPaymentParams struct {
	UserType                 PSEUserType
	UserLegalIDType          LegalIDType // CC or NIT
	UserLegalID              string
	FinancialInstitutionCode string // from PSE.ListFinancialInstitutions
	PaymentDescription       string
	// Optional references: payer IP, product opening date (yyyymmdd) and
	// beneficiary document, as requested by some banks.
	ReferenceOne, ReferenceTwo, ReferenceThree string
}

// PSEPayment builds a PSE bank-transfer payment. Redirect the payer to
// Transaction.AsyncPaymentURL once it is available.
func PSEPayment(p PSEPaymentParams) *PaymentMethod {
	return &PaymentMethod{
		Type:                     PaymentMethodPSE,
		UserType:                 p.UserType,
		UserLegalIDType:          p.UserLegalIDType,
		UserLegalID:              p.UserLegalID,
		FinancialInstitutionCode: p.FinancialInstitutionCode,
		PaymentDescription:       p.PaymentDescription,
		ReferenceOne:             p.ReferenceOne,
		ReferenceTwo:             p.ReferenceTwo,
		ReferenceThree:           p.ReferenceThree,
	}
}

// BancolombiaTransferPayment builds a "Botón Bancolombia" payment. Redirect
// the payer to Transaction.AsyncPaymentURL once it is available.
func BancolombiaTransferPayment(description, ecommerceURL string) *PaymentMethod {
	return &PaymentMethod{
		Type:               PaymentMethodBancolombiaTransfer,
		UserType:           "PERSON",
		PaymentDescription: description,
		EcommerceURL:       ecommerceURL,
	}
}

// BancolombiaCollectPayment builds a cash payment at Bancolombia
// correspondents. Poll the transaction until the business agreement code and
// payment intention identifier appear in PaymentMethod.Extra.
func BancolombiaCollectPayment() *PaymentMethod {
	return &PaymentMethod{Type: PaymentMethodBancolombiaCollect}
}

// BancolombiaQRPayment builds a Bancolombia QR payment. The QR image is
// returned in PaymentMethod.Extra["qr_image"] (base64).
func BancolombiaQRPayment(description string) *PaymentMethod {
	return &PaymentMethod{Type: PaymentMethodBancolombiaQR, PaymentDescription: description}
}

// DaviplataPayment builds a Daviplata payment. Redirect the payer to
// Transaction.AsyncPaymentURL to complete the OTP step.
func DaviplataPayment(legalIDType LegalIDType, legalID, description string) *PaymentMethod {
	return &PaymentMethod{
		Type:               PaymentMethodDaviplata,
		UserLegalIDType:    legalIDType,
		UserLegalID:        legalID,
		PaymentDescription: description,
	}
}

// PCOLPayment builds a Puntos Colombia payment.
func PCOLPayment() *PaymentMethod {
	return &PaymentMethod{Type: PaymentMethodPCOL}
}

// SuPlusPayment builds a SU+ Pay (buy now, pay later) payment.
func SuPlusPayment(legalIDType LegalIDType, legalID string) *PaymentMethod {
	return &PaymentMethod{Type: PaymentMethodSuPlus, UserLegalIDType: legalIDType, UserLegalID: legalID}
}

// BancolombiaBNPLParams are the fields of a Bancolombia "buy now, pay later"
// payment.
type BancolombiaBNPLParams struct {
	Name, LastName     string
	UserLegalIDType    LegalIDType
	UserLegalID        string
	PhoneNumber        string
	PhoneCode          string // e.g. "+57"
	RedirectURL        string
	PaymentDescription string
}

// BancolombiaBNPLPayment builds a Bancolombia BNPL payment.
func BancolombiaBNPLPayment(p BancolombiaBNPLParams) *PaymentMethod {
	return &PaymentMethod{
		Type:               PaymentMethodBancolombiaBNPL,
		Name:               p.Name,
		LastName:           p.LastName,
		UserLegalIDType:    p.UserLegalIDType,
		UserLegalID:        p.UserLegalID,
		PhoneNumber:        p.PhoneNumber,
		PhoneCode:          p.PhoneCode,
		RedirectURL:        p.RedirectURL,
		PaymentDescription: p.PaymentDescription,
	}
}

// requiresTopLevelType reports whether Wompi expects payment_method_type at
// the top level of the transaction body for this method.
func requiresTopLevelType(t PaymentMethodType) bool {
	switch t {
	case PaymentMethodDaviplata, PaymentMethodSuPlus, PaymentMethodBancolombiaBNPL:
		return true
	}
	return false
}
