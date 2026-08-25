package store

import (
	execdomain "github.com/acme/wasm-sandbox-executor/internal/execution/domain"
	moddomain "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

func cloneModule(m moddomain.Module) moddomain.Module {
	return m
}

func cloneExecution(e execdomain.Execution) execdomain.Execution {
	return e
}
