// Google 牛逼！
package interrupt

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func Context() (context.Context, func()) {
	return WrappedContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

func WrappedContext(ctx context.Context, signals ...os.Signal) (context.Context, func()) {
	ctx, closer := context.WithCancel(ctx)

	c := make(chan os.Signal, 1)
	signal.Notify(c, signals...)

	go func() {
		select {
		case <-c:
			closer()
		case <-ctx.Done():
		}
	}()

	return ctx, closer
}
