package search_test

import (
	"testing"

	"github.com/Akagi201/esalert/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestDict(t *testing.T) {
	s := []byte(`
foo: 1
bar:
  baz: buz
  box: wat
biz:
  - something
  - a: 1
    b: 2
  - c: 3
    d: 4`)
	d := search.Dict{}
	require.Nil(t, yaml.Unmarshal(s, &d))
	assert.Equal(t, 1, d["foo"])
	assert.Equal(t, search.Dict{"baz": "buz", "box": "wat"}, d["bar"])
	assert.Equal(t, "something", d["biz"].([]interface{})[0])
	assert.Equal(t, search.Dict{"a": 1, "b": 2}, d["biz"].([]interface{})[1])
	assert.Equal(t, search.Dict{"c": 3, "d": 4}, d["biz"].([]interface{})[2])
}
