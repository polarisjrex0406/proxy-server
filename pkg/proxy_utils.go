package pkg

import "errors"

var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrIPNotFound       = errors.New("ip not found")
	ErrInvalidTargeting = errors.New("invalid targeting")
)

func hasAccess(purchase *Purchase, request *Request) error {
	if len(purchase.IPs) > 0 {
		_, ok := purchase.IPs[request.UserIP]
		if !ok {
			return ErrIPNotFound
		}
	}

	if request.PurchaseType == PurchaseStatic && (request.Country != nil || request.Region != nil || request.City != nil) {
		return ErrInvalidTargeting
	}

	if request.PurchaseType == PurchaseResidential && request.IP != nil {
		return ErrInvalidTargeting
	}

	return nil
}
