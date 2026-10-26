package store

import "context"

type ledgerCtxKey struct{}

// WithLedgerID 将账本编号绑定到请求上下文，后续 store 调用按该账本读写。
func WithLedgerID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ledgerCtxKey{}, id)
}

// LedgerID 返回上下文绑定的账本编号；未绑定或非法时回退到 1，保证直接调用 store 的测试兼容。
func LedgerID(ctx context.Context) int64 {
	if v, ok := ctx.Value(ledgerCtxKey{}).(int64); ok && v > 0 {
		return v
	}
	return 1
}
