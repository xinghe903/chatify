package bo

import "context"

type SendContext struct {
	Ctx  context.Context
	Data []byte
}
