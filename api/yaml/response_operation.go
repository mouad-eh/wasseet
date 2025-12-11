package yaml

import (
	"fmt"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/mouad-eh/wasseet/proxy/config"
)

type IResponseOperation interface {
	Validate() error
	Resolve() config.ResponseOperation
}

type ResponseOperationWrapper struct {
	Operation IResponseOperation
}

func (w *ResponseOperationWrapper) UnmarshalYAML(node *yaml.Node) error {
	var ResponseOp ResponseOperation
	if err := node.Decode(&ResponseOp); err != nil {
		return err
	}

	if ResponseOp.Type == "" {
		return fmt.Errorf("response operation type is missing")
	}

	var op IResponseOperation
	switch ResponseOp.Type {
	case addHeaderResponseOperationType:
		op = &AddHeaderResponseOperation{}
	default:
		return fmt.Errorf("unknown response operation type: %s", ResponseOp.Type)
	}

	if err := node.Decode(op); err != nil {
		return err
	}

	w.Operation = op

	return nil
}

type ResponseOperation struct {
	Type ResponseOperationType `yaml:"type"`
}

type ResponseOperationType string

const (
	addHeaderResponseOperationType ResponseOperationType = "add_header"
)

type AddHeaderResponseOperation struct {
	ResponseOperation
	Header string `yaml:"header"`
	Value  string `yaml:"value"`
}

func (op *AddHeaderResponseOperation) Apply(res *http.Response) {}

func (op *AddHeaderResponseOperation) Validate() error {
	if op.Header == "" {
		return fmt.Errorf("header is missing")
	}
	if op.Value == "" {
		return fmt.Errorf("value is missing")
	}
	return nil
}

func (op *AddHeaderResponseOperation) Resolve() config.ResponseOperation {
	return &config.AddHeaderResponseOperation{
		Header: op.Header,
		Value:  op.Value,
	}
}
