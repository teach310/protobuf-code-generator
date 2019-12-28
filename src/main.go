package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"protoc-gen-model/generator"
	"strings"

	"github.com/golang/protobuf/proto"
	descriptor "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func parseReq(r io.Reader) (*plugin.CodeGeneratorRequest, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var req plugin.CodeGeneratorRequest
	if err = proto.Unmarshal(buf, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// func processReq(req *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
// 	files := make(map[string]*descriptor.FileDescriptorProto)
// 	for _, f := range req.ProtoFile {
// 		files[f.GetName()] = f
// 	}

// 	var resp plugin.CodeGeneratorResponse
// 	for _, fname := range req.FileToGenerate {

// 		f := files[fname]
// 		messageProtos := f.GetMessageType()
// 		messages := make([]map[string]interface{}, 0, len(messageProtos))
// 		for i := 0; i < len(messageProtos); i++ {
// 			messages = append(messages, GetMessage(messageProtos[i]))
// 		}
// 		dataMap := map[string]interface{}{
// 			"filename": fname,
// 			"protofile": map[string]interface{}{
// 				"name":       f.GetName(),
// 				"package":    f.GetPackage(),
// 				"Dependency": f.GetDependency(),
// 				"messages":   messages,
// 			},
// 		}
// 		i, _ := json.MarshalIndent(dataMap, "", "   ")
// 		dataJSON := string(i)
// 		out := fname + ".json"
// 		resp.File = append(resp.File, &plugin.CodeGeneratorResponse_File{
// 			Name:    proto.String(out),
// 			Content: proto.String(dataJSON),
// 		})
// 	}
// 	return &resp
// }

func processReq(req *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {
	files := make(map[string]*descriptor.FileDescriptorProto)
	for _, f := range req.ProtoFile {
		files[f.GetName()] = f
	}

	var resp plugin.CodeGeneratorResponse
	for _, fname := range req.FileToGenerate {
		f := files[fname]

		messageProtos := f.GetMessageType()
		structValues := make([]*generator.StructValue, 0, len(messageProtos))
		for i := 0; i < len(messageProtos); i++ {
			structValues = append(structValues, GetGoStructValue(messageProtos[i]))
		}

		goFile := &generator.File{
			PackageName:  f.GetPackage(), // TODO packageが階層だったときの処理
			StructValues: structValues,
		}
		content := generator.GenerateGoCode(goFile)
		out := strings.Replace(fname, ".proto", ".pb.go", 1)
		resp.File = append(resp.File, &plugin.CodeGeneratorResponse_File{
			Name:    proto.String(out),
			Content: proto.String(content),
		})
	}
	return &resp
}

func GetMessage(messageProto *descriptor.DescriptorProto) map[string]interface{} {
	fieldProtos := messageProto.GetField()
	fields := make([]map[string]interface{}, 0, len(fieldProtos))
	for i := 0; i < len(fieldProtos); i++ {
		fields = append(fields, GetField(fieldProtos[i]))
	}
	message := map[string]interface{}{
		"name": messageProto.GetName(),
	}
	if len(fields) != 0 {
		message["fields"] = fields
	}

	return message
}

func GetGoStructValue(messageProto *descriptor.DescriptorProto) *generator.StructValue {
	fieldProtos := messageProto.GetField()
	fields := make([]*generator.StructField, 0, len(fieldProtos))
	for i := 0; i < len(fieldProtos); i++ {
		fields = append(fields, GetGoStructField(fieldProtos[i]))
	}
	structValue := &generator.StructValue{
		Name:   messageProto.GetName(),
		Fields: fields,
	}

	return structValue
}

func GetGoStructField(fieldProto *descriptor.FieldDescriptorProto) *generator.StructField {
	label := fieldProto.GetLabel()
	typ, err := GetGoType(fieldProto.GetType(), fieldProto.GetTypeName(), label)
	if err != nil {
		panic(err)
	}

	field := &generator.StructField{
		Name: fieldProto.GetName(),
		Type: typ,
	}

	return field
}

func GetGoType(typeProto descriptor.FieldDescriptorProto_Type, typeName string, label descriptor.FieldDescriptorProto_Label) (*generator.Type, error) {
	// 独自クラスかそれ以外で分ける
	goTypeName := ""
	switch typeProto {
	case descriptor.FieldDescriptorProto_TYPE_INT32:
		goTypeName = "int"
	case descriptor.FieldDescriptorProto_TYPE_INT64:
		goTypeName = "int64"
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		goTypeName = "bool"
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		goTypeName = "string"
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		goTypeName = GetGoStructTypeName(typeName)
	default:
		return nil, fmt.Errorf("GetGoType unsupported type error %v", typeProto)
	}

	if label == descriptor.FieldDescriptorProto_LABEL_REPEATED {
		goTypeName = "[]" + goTypeName
	}
	// TODO map
	return &generator.Type{Name: goTypeName}, nil
}

func GetGoStructTypeName(typeName string) string {
	if typeName == ".google.protobuf.Timestamp" {
		return "time.Time"
	}
	// キャッシュで高速化できる
	typeSlice := strings.Split(typeName, ".")
	return typeSlice[len(typeSlice)-1]
}

func GetField(fieldProto *descriptor.FieldDescriptorProto) map[string]interface{} {
	label := fieldProto.GetLabel()
	field := map[string]interface{}{
		"name":     fieldProto.GetName(),
		"type":     fieldProto.GetType(), // 11ならmessage
		"typename": fieldProto.GetTypeName(),
		"repeated": label == descriptor.FieldDescriptorProto_LABEL_REPEATED,
	}

	return field
}

func emitResp(resp *plugin.CodeGeneratorResponse) error {
	buf, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(buf)
	return err
}

func run() error {
	req, err := parseReq(os.Stdin)
	if err != nil {
		return err
	}

	resp := processReq(req)

	return emitResp(resp)
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}