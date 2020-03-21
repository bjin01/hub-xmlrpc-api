package server

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/rpc"
	"github.com/kolo/xmlrpc"
)

type Codec struct {
	methods                  map[string]string
	defaultMethodByNamespace map[string]string
	defaultMethod            string
	parsers                  map[string]parser
	defaultParser            parser
}

func NewCodec() *Codec {
	return &Codec{
		methods:                  make(map[string]string),
		defaultMethodByNamespace: make(map[string]string),
		defaultMethod:            "",
		parsers:                  make(map[string]parser),
		defaultParser:            nil,
	}
}

func (c *Codec) RegisterDefaultParser(parser parser) {
	c.defaultParser = parser
}

func (c *Codec) RegisterMethod(method string) {
	c.methods[method] = method
}

func (c *Codec) RegisterMethodWithParser(method string, parser parser) {
	c.methods[method] = method
	c.parsers[c.resolveMethod(method)] = parser
}

func (c *Codec) RegisterDefaultMethod(method string, parser parser) {
	c.defaultMethod = method
	c.parsers[c.resolveMethod(method)] = parser
}

func (c *Codec) RegisterDefaultMethodForNamespace(namespace, method string, parser parser) {
	c.defaultMethodByNamespace[namespace] = method
	c.parsers[c.resolveMethod(method)] = parser
}

func (c *Codec) NewRequest(r *http.Request) rpc.CodecRequest {
	rawxml, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return &CodecRequest{err: err}
	}
	defer r.Body.Close()

	r.Body = ioutil.NopCloser(bytes.NewBuffer(rawxml))

	var request ServerRequest
	if err := xml.Unmarshal(rawxml, &request); err != nil {
		return &CodecRequest{err: err}
	}
	request.rawxml = rawxml
	request.Method = c.resolveMethod(request.Method)

	parser := c.resolveParser(request.Method)

	return &CodecRequest{request: &request, parser: parser}
}

func (c *Codec) resolveParser(requestMethod string) parser {
	if parser, ok := c.parsers[requestMethod]; ok {
		return parser
	}
	return c.defaultParser
}

func (c *Codec) resolveMethod(requestMethod string) string {
	namespace, methodStr := c.getNamespaceAndMethod(requestMethod)
	if _, ok := c.methods[requestMethod]; ok {
		return c.toLowerCase(namespace, methodStr)
	} else if method, ok := c.defaultMethodByNamespace[namespace]; ok {
		return method
	} else if c.defaultMethod != "" {
		return c.defaultMethod
	}
	return requestMethod
}

func (c *Codec) getNamespaceAndMethod(requestMethod string) (string, string) {
	//TODO:
	if len(requestMethod) > 1 {
		parts := strings.Split(requestMethod, ".")
		slice := parts[1:len(parts)]
		return parts[0], strings.Join(slice, ".")
	}
	return "", ""
}

func (c *Codec) toLowerCase(namespace, method string) string {
	//TODO:
	if namespace != "" && method != "" {
		r, n := utf8.DecodeRuneInString(method)
		if unicode.IsLower(r) {
			return namespace + "." + string(unicode.ToUpper(r)) + method[n:]
		}
	}
	return namespace + "." + method
}

type ServerRequest struct {
	Name   xml.Name `xml:"methodCall"`
	Method string   `xml:"methodName"`
	rawxml []byte
}

type CodecRequest struct {
	request *ServerRequest
	err     error
	parser  parser
}

func (c *CodecRequest) Method() (string, error) {
	if c.err == nil {
		return c.request.Method, nil
	}
	return "", c.err
}

func (c *CodecRequest) ReadRequest(args interface{}) error {
	val := reflect.ValueOf(args)
	if val.Kind() != reflect.Ptr {
		return errors.New("non-pointer value passed")
	}

	var argsList []interface{}
	argsList, c.err = xmlrpc.UnmarshalToList(c.request.rawxml)
	if c.err != nil {
		return c.err
	}

	err := c.parser(argsList, args)
	if err != nil {
		return err
	}
	return nil
}

func (c *CodecRequest) WriteResponse(w http.ResponseWriter, response interface{}, methodErr error) error {
	var xmlstr string
	if c.err != nil {
		//TODO:
		/*	var fault Fault
			switch c.err.(type) {
			case Fault:
				fault = c.err.(Fault)
			default:
				fault = FaultApplicationError
				fault.String += fmt.Sprintf(": %v", c.err)
			}
			xmlstr = fault2XML(fault)*/
		return c.err
	} else if methodErr != nil {

		var fault Fault
		switch methodErr.(type) {
		case Fault:
			fault = methodErr.(Fault)
		default:
			fault = FaultApplicationError
			fault.String += fmt.Sprintf(": %v", methodErr)
		}
		xmlstr = fault2XML(fault)
	} else {
		xmlstr, _ = encodeResponseToXML(response)
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Write([]byte(xmlstr))
	return nil
}
