package injector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	jsonPriorityTag  = "json:\"priority\""
	validateRangeTag = "validate:\"gte=1,lte=100\""
	gotagsMarker     = "@gotags:"

	weightField   = "\tWeight int32 `protobuf:\"varint,8,opt,name=weight,proto3\" json:\"weight,omitempty\"`"
	nameField     = "\tName string `protobuf:\"bytes,1,opt,name=name,proto3\" json:\"name,omitempty\"`"
	scoreField    = "\tScore int32 `protobuf:\"varint,1,opt,name=score,proto3\"`"
	idField       = "\tID string `protobuf:\"bytes,1,opt,name=id,proto3\"`"
	settingsField = "\tSettings *UserSettings `protobuf:\"bytes,10,opt,name=settings,proto3\" json:\"settings,omitempty\"`"
	sliceField    = "\tRoles []string `protobuf:\"bytes,12,rep,name=roles,proto3\" json:\"roles,omitempty\"`"
	mapField      = "\tMetadata map[string]string `protobuf:\"bytes,11,rep,name=metadata,proto3\" json:\"metadata,omitempty\" protobuf_key:\"bytes,1,opt,name=key\" protobuf_val:\"bytes,2,opt,name=value\"`"
)

type expect struct {
	changed     bool
	contains    []string
	notContains []string
	equalInput  bool
}

func makeStruct(lines ...string) []byte {
	return []byte("type X struct {\n" + strings.Join(lines, "\n") + "\n}\n")
}

func makeFile(lines ...string) string {
	return "package pb\n\n" + string(makeStruct(lines...))
}

func withComment(field, comment string) string {
	return field + " // " + comment
}

func runInject(t *testing.T, input []byte) (string, bool) {
	t.Helper()
	out, changed, err := New(Options{}).injectTags(input)
	require.NoError(t, err)
	return string(out), changed
}

func assertOutput(t *testing.T, got string, input []byte, ex expect) {
	t.Helper()
	for _, token := range ex.contains {
		assert.Contains(t, got, token)
	}
	for _, token := range ex.notContains {
		assert.NotContains(t, got, token)
	}
	if ex.equalInput {
		assert.Equal(t, string(input), got)
	}
}

func TestInjectTagsCases(t *testing.T) {
	type testCase struct {
		input  []byte
		expect expect
	}

	cases := map[string]testCase{
		"single comment gotags": {
			input:  makeStruct(withComment(weightField, "Priority weight "+gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag)),
			expect: expect{changed: true, contains: []string{jsonPriorityTag, validateRangeTag}, notContains: []string{gotagsMarker}},
		},
		"double slash gotags": {
			input:  makeStruct(withComment(weightField, "Priority weight // "+gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag)),
			expect: expect{changed: true, contains: []string{jsonPriorityTag, validateRangeTag}, notContains: []string{gotagsMarker}},
		},
		"inject tags alias": {
			input:  makeStruct(withComment(scoreField, "@inject_tags: json:\"score\" validate:\"gte=0,lte=100\"")),
			expect: expect{changed: true, contains: []string{"json:\"score\"", "validate:\"gte=0,lte=100\""}},
		},
		"inject tag singular alias": {
			input:  makeStruct(withComment(scoreField, "@inject_tag: json:\"score\" validate:\"gte=0,lte=100\"")),
			expect: expect{changed: true, contains: []string{"json:\"score\"", "validate:\"gte=0,lte=100\""}},
		},
		"gotag singular alias": {
			input:  makeStruct(withComment(idField, "@gotag: json:\"id\" validate:\"required\"")),
			expect: expect{changed: true, contains: []string{"json:\"id\"", "validate:\"required\""}},
		},
		"overwrite existing json keep other tags": {
			input:  makeStruct(withComment("\tWeight int32 `protobuf:\"varint,8,opt,name=weight,proto3\" json:\"weight,omitempty\" bson:\"weight\"`", gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag)),
			expect: expect{changed: true, contains: []string{jsonPriorityTag, validateRangeTag, "bson:\"weight\"", "protobuf:\"varint,8,opt,name=weight,proto3\""}},
		},
		"invalid new tag no change": {
			input:  makeStruct(withComment(weightField, gotagsMarker+" json:\"priority validate:\"gte=1,lte=100\"")),
			expect: expect{changed: false, equalInput: true},
		},
		"two fields only one changes": {
			input:  makeStruct(nameField, withComment("\tWeight int32 `protobuf:\"varint,2,opt,name=weight,proto3\" json:\"weight,omitempty\"`", gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag)),
			expect: expect{changed: true, contains: []string{nameField, jsonPriorityTag, validateRangeTag}},
		},
		"nested struct pointer field": {
			input:  makeStruct(withComment(settingsField, gotagsMarker+" json:\"settings\" validate:\"omitempty\"")),
			expect: expect{changed: true, contains: []string{"Settings *UserSettings", "json:\"settings\"", "validate:\"omitempty\""}},
		},
		"slice field": {
			input:  makeStruct(withComment(sliceField, gotagsMarker+" json:\"roles\" validate:\"omitempty,dive,oneof=admin user guest\"")),
			expect: expect{changed: true, contains: []string{"Roles []string", "json:\"roles\"", "validate:\"omitempty,dive,oneof=admin user guest\""}},
		},
		"map field": {
			input:  makeStruct(withComment(mapField, gotagsMarker+" json:\"metadata\" validate:\"omitempty\"")),
			expect: expect{changed: true, contains: []string{"Metadata map[string]string", "json:\"metadata\"", "validate:\"omitempty\"", "protobuf_key:\"bytes,1,opt,name=key\"", "protobuf_val:\"bytes,2,opt,name=value\""}},
		},
		"no marker no change": {
			input:  makeStruct(withComment(weightField, "Priority weight")),
			expect: expect{changed: false, equalInput: true},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, changed := runInject(t, tc.input)
			assert.Equal(t, tc.expect.changed, changed)
			assertOutput(t, got, tc.input, tc.expect)
		})
	}
}

func TestRemoveGotagsCommentsCases(t *testing.T) {
	type testCase struct {
		input       []byte
		contains    []string
		notContains []string
	}

	cases := map[string]testCase{
		"remove inline gotags keep prefix comment": {
			input:       []byte("// Priority " + gotagsMarker + " " + jsonPriorityTag + " " + validateRangeTag + "\n"),
			contains:    []string{"// Priority"},
			notContains: []string{gotagsMarker},
		},
		"remove standalone gotags line": {
			input:       makeStruct("\t// "+gotagsMarker+" "+jsonPriorityTag, "\tWeight int32 `json:\"weight\"`"),
			contains:    []string{"Weight int32 `json:\"weight\"`"},
			notContains: []string{gotagsMarker},
		},
		"remove inject_tags marker too": {
			input:       []byte("// note @inject_tags: validate:\"gte=0,lte=100\"\n"),
			contains:    []string{"// note"},
			notContains: []string{"@inject_tags:"},
		},
		"trim extra blank lines": {
			input:       makeStruct("\t// "+gotagsMarker+" json:\"x\"", "", "", "\tName string"),
			contains:    []string{"Name string"},
			notContains: []string{"\n\n\n", gotagsMarker},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := string(New(Options{}).removeGotagsComments(tc.input))
			for _, token := range tc.contains {
				assert.Contains(t, got, token)
			}
			for _, token := range tc.notContains {
				assert.NotContains(t, got, token)
			}
		})
	}
}

func TestProcessFileEndToEnd(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "x.pb.go")
	input := makeFile(withComment(weightField, "Priority "+gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag))
	require.NoError(t, os.WriteFile(file, []byte(input), 0644))

	inj := New(Options{RemoveComments: true, FormatCode: true})
	require.NoError(t, inj.ProcessFile(file))

	out, err := os.ReadFile(file)
	require.NoError(t, err)
	got := string(out)
	assert.Contains(t, got, jsonPriorityTag)
	assert.Contains(t, got, validateRangeTag)
	assert.NotContains(t, got, gotagsMarker)
}

func TestProcessFileDryRunDoesNotModify(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "x.pb.go")
	input := makeFile(withComment(weightField, "Priority "+gotagsMarker+" "+jsonPriorityTag+" "+validateRangeTag))
	require.NoError(t, os.WriteFile(file, []byte(input), 0644))

	inj := New(Options{DryRun: true, RemoveComments: true, FormatCode: true})
	require.NoError(t, inj.ProcessFile(file))

	out, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, input, string(out))
}

func TestProcessFileNotFound(t *testing.T) {
	t.Parallel()
	err := New(Options{}).ProcessFile(filepath.Join(t.TempDir(), "not_exists.pb.go"))
	require.Error(t, err)
}
