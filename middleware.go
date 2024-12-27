package telnet

import (
	"slices"
	"sync"
)

type Middleware interface {
	Handle(terminal *Terminal, data TerminalData, next TerminalDataHandler)
}

type MiddlewareStack struct {
	lineOut TerminalDataHandler

	middlewareLock sync.RWMutex

	middlewares        []Middleware
	middlewareWrappers []TerminalDataHandler
}

func NewMiddlewareStack(lineOut TerminalDataHandler, middlewares ...Middleware) *MiddlewareStack {
	stack := &MiddlewareStack{
		lineOut: lineOut,
	}

	stack.middlewares = middlewares

	if len(middlewares) > 0 {
		stack.middlewareWrappers = make([]TerminalDataHandler, len(middlewares))
		stack.middlewareWrappers[len(stack.middlewareWrappers)-1] = func(t *Terminal, data TerminalData) {
			stack.middlewares[len(stack.middlewares)-1].Handle(t, data, stack.lineOut)
		}
	}

	if len(middlewares) > 1 {
		stack.rebuildMiddlewares(len(stack.middlewares) - 2)
	}

	return stack
}

func (s *MiddlewareStack) PushMiddleware(middleware Middleware) {
	s.middlewareLock.Lock()
	defer s.middlewareLock.Unlock()

	oldTop := s.lineOut
	if len(s.middlewareWrappers) > 0 {
		oldTop = s.middlewareWrappers[0]
	}
	s.middlewares = slices.Insert(s.middlewares, 0, middleware)
	s.middlewareWrappers = slices.Insert(s.middlewareWrappers, 0, func(t *Terminal, data TerminalData) {
		middleware.Handle(t, data, oldTop)
	})
}

func (s *MiddlewareStack) rebuildMiddlewares(endIndex int) {
	for i := endIndex; i >= 0; i-- {
		s.middlewareWrappers[i] = func(t *Terminal, data TerminalData) {
			s.middlewares[i].Handle(t, data, s.middlewareWrappers[i+1])
		}
	}
}

func (s *MiddlewareStack) QueueMiddleware(middleware Middleware) {
	s.middlewareLock.Lock()
	defer s.middlewareLock.Unlock()

	s.middlewares = append(s.middlewares, middleware)
	s.middlewareWrappers = append(s.middlewareWrappers, func(t *Terminal, data TerminalData) {
		middleware.Handle(t, data, s.lineOut)
	})

	s.rebuildMiddlewares(len(s.middlewares) - 2)
}

func (s *MiddlewareStack) RemoveMiddleware(middleware Middleware) {
	s.middlewareLock.Lock()
	defer s.middlewareLock.Unlock()

	middlewareIndex := -1
	for i := 0; i < len(s.middlewares); i++ {
		if s.middlewares[i] == middleware {
			middlewareIndex = i
			break
		}
	}

	if middlewareIndex < 0 {
		return
	}

	s.middlewares = slices.Delete(s.middlewares, middlewareIndex, middlewareIndex+1)
	s.middlewareWrappers = slices.Delete(s.middlewareWrappers, middlewareIndex, middlewareIndex+1)

	if len(s.middlewares) == 0 {
		return
	}

	if middlewareIndex >= len(s.middlewares) {
		middlewareIndex = len(s.middlewares) - 1

		// We deleted the last item so the new last item needs to be rigged up to lineout
		s.middlewareWrappers[middlewareIndex] = func(t *Terminal, data TerminalData) {
			s.middlewares[middlewareIndex].Handle(t, data, s.lineOut)
		}

	}

	s.rebuildMiddlewares(middlewareIndex - 1)
}

func (s *MiddlewareStack) LineIn(t *Terminal, data TerminalData) {
	s.middlewareLock.RLock()
	defer s.middlewareLock.RUnlock()

	if len(s.middlewares) == 0 {
		s.lineOut(t, data)
		return
	}

	s.middlewareWrappers[0](t, data)
}
