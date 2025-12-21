package yaml

import (
	"fmt"

	"github.com/mouad-eh/wasseet/api/config"
	"github.com/mouad-eh/wasseet/request"
	"gopkg.in/yaml.v3"
)

type IRequestOperation interface {
	Validate() error
	Resolve() config.RequestOperation
}

type RequestOperationWrapper struct {
	Operation IRequestOperation
}

func (w *RequestOperationWrapper) UnmarshalYAML(node *yaml.Node) error {
	var RequestOp RequestOperation
	if err := node.Decode(&RequestOp); err != nil {
		return err
	}

	if RequestOp.Type == "" {
		return fmt.Errorf("request operation type is missing")
	}

	var op IRequestOperation
	switch RequestOp.Type {
	case addHeaderRequestOperationType:
		op = &AddHeaderRequestOperation{}
	default:
		return fmt.Errorf("unknown request operation type: %s", RequestOp.Type)
	}

	if err := node.Decode(op); err != nil {
		return err
	}

	w.Operation = op

	return nil
}

type RequestOperation struct {
	Type RequestOperationType `yaml:"type"`
}

type RequestOperationType string

const (
	addHeaderRequestOperationType RequestOperationType = "add_header"
)

type AddHeaderRequestOperation struct {
	RequestOperation
	Header string `yaml:"header"`
	Value  string `yaml:"value"`
}

func (op *AddHeaderRequestOperation) Apply(req request.ServerRequest) {}

func (op *AddHeaderRequestOperation) Validate() error {
	if op.Header == "" {
		return fmt.Errorf("header is missing")
	}
	if op.Value == "" {
		return fmt.Errorf("value is missing")
	}
	return nil
}

func (op *AddHeaderRequestOperation) Resolve() config.RequestOperation {
	return &config.AddHeaderRequestOperation{
		Header: op.Header,
		Value:  op.Value,
	}
}
