package service

import "context"

type actorContext struct{}

// ContextWithActor annotates the context with the actor user ID for auditing/event emission.
func ContextWithActor(ctx context.Context, actor string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, actorContext{}, actor)
}

// actorFromContext extracts the actor user ID if present.
func actorFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(actorContext{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
