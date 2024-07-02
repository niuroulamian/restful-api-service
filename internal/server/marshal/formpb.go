// Package marshal is to parse application/x-www-form-urlencoded and return json
// You should add `runtime.WithMarshalerOption("application/x-www-form-urlencoded", &runtime.FORMPb{}),`
// before MIMEWildcard
package marshal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoiface"
)

// Form implements customized marshaler for incoming request with type x-www-form-urlencoded
type Form struct {
	PB *runtime.JSONPb
}

// Unmarshal returns result of builtin method of runtime.JSONPb
func (f *Form) Unmarshal(data []byte, v interface{}) error {
	return f.PB.Unmarshal(data, v)
}

// NewEncoder returns result of builtin method of runtime.JSONPb
func (f *Form) NewEncoder(w io.Writer) runtime.Encoder {
	return f.PB.NewEncoder(w)
}

// ContentType implements customized marshaler for incoming request with type x-www-form-urlencoded
func (f *Form) ContentType() string {
	return "application/x-www-form-urlencoded"
}

// Marshal implements customized marshaler for incoming request with type x-www-form-urlencoded
func (f *Form) Marshal(v interface{}) ([]byte, error) {
	if _, ok := v.(proto.Message); !ok {
		return f.marshalNonProtoField(v)
	}

	buf, err := f.marshalTo(v)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
func (f *Form) marshalTo(v interface{}) ([]byte, error) {
	p, ok := v.(proto.Message)
	if !ok {
		buf, err := f.marshalNonProtoField(v)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
	return f.PB.Marshal(p)
}

type protoEnum interface {
	fmt.Stringer
	EnumDescriptor() ([]byte, []int)
}

func (f *Form) marshalNonProtoField(v interface{}) ([]byte, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte("null"), nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Map {
		m := make(map[string]*json.RawMessage)
		for _, k := range rv.MapKeys() {
			buf, err := f.PB.Marshal(rv.MapIndex(k).Interface())
			if err != nil {
				return nil, err
			}
			m[fmt.Sprintf("%v", k.Interface())] = (*json.RawMessage)(&buf)
		}
		if f.PB.Indent != "" {
			return json.MarshalIndent(m, "", f.PB.Indent)
		}
		return json.Marshal(m)
	}
	if enum, ok := rv.Interface().(protoEnum); ok && !f.PB.EnumsAsInts {
		return json.Marshal(enum.String())
	}
	return json.Marshal(rv.Interface())
}

// NewDecoder implements customized marshaler for incoming request with type x-www-form-urlencoded
func (f *Form) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(v interface{}) error { return decodeFORMPb(r, v) })
}

func decodeFORMPb(d io.Reader, v interface{}) error {
	msg, ok := v.(protoiface.MessageV1)

	if !ok {
		return fmt.Errorf("not proto message")
	}

	formData, err := io.ReadAll(d)

	if err != nil {
		return err
	}

	values, err := url.ParseQuery(string(formData))

	if err != nil {
		return err
	}

	filter := &utilities.DoubleArray{}

	err = runtime.PopulateQueryParameters(msg, values, filter)

	if err != nil {
		return err
	}

	return nil
}
