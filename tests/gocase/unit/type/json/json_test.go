/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package json

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/apache/kvrocks/tests/gocase/util"
	"github.com/stretchr/testify/require"
)

func TestJson(t *testing.T) {
	srv := util.StartServer(t, map[string]string{})
	defer srv.Close()
	ctx := context.Background()
	rdb := srv.NewClient()
	defer func() { require.NoError(t, rdb.Close()) }()

	t.Run("JSON.SET and JSON.GET basics", func(t *testing.T) {
		require.Error(t, rdb.Do(ctx, "JSON.SET", "a").Err())
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":1, "y":2} `).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a").Val(), `{"x":1,"y":2}`)

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$.y", `233`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a").Val(), `{"x":1,"y":233}`)

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[[1], [2]]`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$[*][0]", "3").Err())
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a").Val(), `[[3],[3]]`)

		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$").Val(), `[[[3],[3]]]`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$[0]").Val(), `[[3]]`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$[0][0]").Val(), `[3]`)

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"x":1,"y":{"x":{"y":2},"y":3}}`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a").Val(), `{"x":1,"y":{"x":{"y":2},"y":3}}`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$").Val(), `[{"x":1,"y":{"x":{"y":2},"y":3}}]`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$..x").Val(), `[1,{"y":2}]`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "$..x", "$..y").Val(), `{"$..x":[1,{"y":2}],"$..y":[{"x":{"y":2},"y":3},3,2]}`)

		require.Equal(t, rdb.Do(ctx, "JSON.GET", "no-such-key").Val(), nil)
		require.Equal(t, rdb.Type(ctx, "a").Val(), "ReJSON-RL")
	})

	t.Run("JSON.DEL and JSON.FORGET basics", func(t *testing.T) {
		// JSON.DEL and JSON.FORGET are aliases
		for _, command := range []string{"JSON.DEL", "JSON.FORGET"} {
			require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"x": 1, "nested": {"x": 2, "y": 3}}`).Err())
			require.EqualValues(t, 2, rdb.Do(ctx, command, "a", "$..x").Val())
			require.Equal(t, `[{"nested":{"y":3}}]`, rdb.Do(ctx, "JSON.GET", "a", "$").Val())

			require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"x": 1, "nested": {"x": 2, "y": 3}}`).Err())
			require.EqualValues(t, 1, rdb.Do(ctx, command, "a", "$.x").Val())
			require.Equal(t, `[{"nested":{"x":2,"y":3}}]`, rdb.Do(ctx, "JSON.GET", "a", "$").Val())

			require.EqualValues(t, 1, rdb.Do(ctx, command, "a", "$").Val())
			require.EqualValues(t, 0, rdb.Do(ctx, command, "no-such-json-key", "$").Val())
		}
	})

	t.Run("JSON.GET with options", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":1, "y":2} `).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "INDENT", " ").Val(), `{ "x":1, "y":2}`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "INDENT", " ", "SPACE", " ").Val(), `{ "x": 1, "y": 2}`)
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "NEWLINE", "\n").Val(), "{\n\"x\":1,\n\"y\":2\n}")
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "NEWLINE", "\n", "INDENT", " ", "SPACE", " ").Val(), "{\n \"x\": 1,\n \"y\": 2\n}")
		require.Equal(t, rdb.Do(ctx, "JSON.GET", "a", "INDENT", " ", "$").Val(), `[ {  "x":1,  "y":2 }]`)
	})

	t.Run("JSON storage format CBOR", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"x":1, "y":2}`).Err())
		require.Equal(t, "json", rdb.Do(ctx, "JSON.INFO", "a").Val().([]interface{})[1])

		require.NoError(t, rdb.Do(ctx, "CONFIG", "SET", "json-storage-format", "cbor").Err())
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "b", "$", `{"x":1, "y":2}`).Err())
		require.Equal(t, "cbor", rdb.Do(ctx, "JSON.INFO", "b").Val().([]interface{})[1])
		require.Equal(t, `{"x":1,"y":2}`, rdb.Do(ctx, "JSON.GET", "b").Val())
		require.Equal(t, `{"x":1,"y":2}`, rdb.Do(ctx, "JSON.GET", "a").Val())
	})

	t.Run("JSON.ARRAPPEND basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "SET", "a", `1`).Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$", `1`).Err())
		require.NoError(t, rdb.Do(ctx, "DEL", "a").Err())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":1, "y": {"x":1} } `).Err())
		require.Equal(t, []interface{}{}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$..k", `1`).Val())
		require.Error(t, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$").Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$", ` 1, 2, 3`).Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$", `1`, ` 1, 2, 3`).Err())
		require.Equal(t, []interface{}{nil, nil}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$..x", `1`).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":1, "y": {"x":[]} } `).Err())
		require.Equal(t, []interface{}{int64(1), nil}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$..x", `1`).Val())
		require.Equal(t, `[{"x":1,"y":{"x":[1]}}]`, rdb.Do(ctx, "JSON.GET", "a", "$").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":[], "y":[]} `).Err())
		require.Equal(t, []interface{}{int64(1)}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$.x", `1`).Val())
		require.Equal(t, `{"x":[1],"y":[]}`, rdb.Do(ctx, "JSON.GET", "a").Val())
		require.Equal(t, []interface{}{int64(4)}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$.x", `1`, `2`, `3`).Val())
		require.Equal(t, []interface{}{int64(1)}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$.y", ` {"x":[], "y":[]} `).Val())
		require.Equal(t, `[{"x":[1,1,2,3],"y":[{"x":[],"y":[]}]}]`, rdb.Do(ctx, "JSON.GET", "a", "$").Val())

		require.Equal(t, []interface{}{int64(2), int64(6)}, rdb.Do(ctx, "JSON.ARRAPPEND", "a", "$..x", `1`, `2`).Val())
		require.Equal(t, `[[1,2]]`, rdb.Do(ctx, "JSON.GET", "a", "$.y[0].x").Val())
		require.Equal(t, `[]`, rdb.Do(ctx, "JSON.GET", "a", "$.x.x").Val())
		require.Equal(t, `[{"x":[1,1,2,3,1,2],"y":[{"x":[1,2],"y":[]}]}]`, rdb.Do(ctx, "JSON.GET", "a", "$").Val())
	})

	t.Run("JSON.TYPE basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"b":true,"x":1, "y":1.2, "z": {"x":[1,2,3], "y": null}, "v":{"x":"y"}}`).Err())

		types, err := rdb.Do(ctx, "JSON.TYPE", "a").StringSlice()
		require.NoError(t, err)
		require.Equal(t, []string{"object"}, types)

		types, err = rdb.Do(ctx, "JSON.TYPE", "a", "$").StringSlice()
		require.NoError(t, err)
		require.Equal(t, []string{"object"}, types)

		types, err = rdb.Do(ctx, "JSON.TYPE", "a", "$..x").StringSlice()
		require.NoError(t, err)
		require.EqualValues(t, []string{"integer", "string", "array"}, types)

		types, err = rdb.Do(ctx, "JSON.TYPE", "a", "$..y").StringSlice()
		require.NoError(t, err)
		require.EqualValues(t, []string{"number", "null"}, types)

		types, err = rdb.Do(ctx, "JSON.TYPE", "a", "$.b").StringSlice()
		require.NoError(t, err)
		require.EqualValues(t, []string{"boolean"}, types)

		types, err = rdb.Do(ctx, "JSON.TYPE", "a", "$.no_exists").StringSlice()
		require.NoError(t, err)
		require.EqualValues(t, []string{}, types)

		_, err = rdb.Do(ctx, "JSON.TYPE", "not_exists", "$").StringSlice()
		require.EqualError(t, err, redis.Nil.Error())
	})

	t.Run("JSON.STRAPPEND basics", func(t *testing.T) {
		var result1 = make([]interface{}, 0)
		result1 = append(result1, int64(5))
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":"foo", "nested": {"a": "hello"}, "nested2": {"a": 31}}`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.STRAPPEND", "a", "$.a", "\"be\"").Val(), result1)

		var result2 = make([]interface{}, 0)
		result2 = append(result2, int64(5), int64(7), interface{}(nil))
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":"foo", "nested": {"a": "hello"}, "nested2": {"a": 31}}`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.STRAPPEND", "a", "$..a", "\"be\"").Val(), result2)
	})

	t.Run("JSON.STRLEN basics", func(t *testing.T) {
		var result1 = make([]interface{}, 0)
		result1 = append(result1, int64(3))
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":"foo", "nested": {"a": "hello"}, "nested2": {"a": 31}}`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.STRLEN", "a", "$.a").Val(), result1)

		var result2 = make([]interface{}, 0)
		result2 = append(result2, int64(3), int64(5), interface{}(nil))
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":"foo", "nested": {"a": "hello"}, "nested2": {"a": 31}}`).Err())
		require.Equal(t, rdb.Do(ctx, "JSON.STRLEN", "a", "$..a").Val(), result2)
	})

	t.Run("Merge basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "key", "$", `{"a":2}`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.MERGE", "key", "$.a", `3`).Err())
		require.Equal(t, `{"a":3}`, rdb.Do(ctx, "JSON.GET", "key").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "key", "$", `{"a":2}`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.MERGE", "key", "$.a", `null`).Err())
		require.Equal(t, `{}`, rdb.Do(ctx, "JSON.GET", "key").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "key", "$", `{"a":[2,4,6,8]}`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.MERGE", "key", "$.a", `[10,12]`).Err())
		require.Equal(t, `{"a":[10,12]}`, rdb.Do(ctx, "JSON.GET", "key").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "key", "$", `{"f1": {"a":1}, "f2":{"a":2}}`).Err())
		require.Equal(t, `{"f1":{"a":1},"f2":{"a":2}}`, rdb.Do(ctx, "JSON.GET", "key").Val())
		require.NoError(t, rdb.Do(ctx, "JSON.MERGE", "key", "$", `{"f1": null, "f2":{"a":3, "b":4}, "f3":[2,4,6]}`).Err())
		require.Equal(t, `{"f2":{"a":3,"b":4},"f3":[2,4,6]}`, rdb.Do(ctx, "JSON.GET", "key").Val())
	})

	t.Run("Clear JSON values", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "bb", "$", `{"obj":{"a":1, "b":2}, "arr":[1,2,3], "str": "foo", "bool": true, "int": 42, "float": 3.14}`).Err())

		require.NoError(t, rdb.Do(ctx, "JSON.CLEAR", "bb", "$").Err())
		require.Equal(t, `{}`, rdb.Do(ctx, "JSON.GET", "bb").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "bb", "$", `{"obj":{"a":1, "b":2}, "arr":[1,2,3], "str": "foo", "bool": true, "int": 42, "float": 3.14}`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.CLEAR", "bb", "$.obj").Err())
		require.Equal(t, `{"arr":[1,2,3],"bool":true,"float":3.14,"int":42,"obj":{},"str":"foo"}`, rdb.Do(ctx, "JSON.GET", "bb").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.CLEAR", "bb", "$.arr").Err())
		require.Equal(t, `{"arr":[],"bool":true,"float":3.14,"int":42,"obj":{},"str":"foo"}`, rdb.Do(ctx, "JSON.GET", "bb").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "bb", "$", `{"obj":{"a":1, "b":2}, "arr":[1,2,3], "str": "foo", "bool": true, "int": 42, "float": 3.14}`).Err())
		require.NoError(t, rdb.Do(ctx, "JSON.CLEAR", "bb", "$.*").Err())
		require.Equal(t, `{"arr":[],"bool":true,"float":0,"int":0,"obj":{},"str":"foo"}`, rdb.Do(ctx, "JSON.GET", "bb").Val())

		_, err := rdb.Do(ctx, "JSON.CLEAR", "bb", "$.some").Result()
		require.NoError(t, err)
	})

	t.Run("JSON.ARRLEN basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "DEL", "a").Err())
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a1":[1,2],"a2":[[1,5,7],[8],[9]],"i":1,"d":1.0,"s":"string","o":{"a3":[1,1,1]}}`).Err())

		lens, err := rdb.Do(ctx, "JSON.ARRLEN", "a", "$.a1").Uint64Slice()
		require.NoError(t, err)
		require.EqualValues(t, []uint64{2}, lens)

		lens, err = rdb.Do(ctx, "JSON.ARRLEN", "a", "$.a2").Uint64Slice()
		require.NoError(t, err)
		require.EqualValues(t, []uint64{3}, lens)

		lens, err = rdb.Do(ctx, "JSON.ARRLEN", "a", "$.a2[0]").Uint64Slice()
		require.NoError(t, err)
		require.EqualValues(t, []uint64{3}, lens)

		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRLEN", "a", "$.i").Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRLEN", "a", "$.d").Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRLEN", "a", "$.s").Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRLEN", "a", "$.o").Val())

		lens, err = rdb.Do(ctx, "JSON.ARRLEN", "a", "$.o.a3").Uint64Slice()
		require.NoError(t, err)
		require.EqualValues(t, []uint64{3}, lens)

		_, err = rdb.Do(ctx, "JSON.ARRLEN", "not_exists", "$.*").Uint64Slice()
		require.EqualError(t, err, redis.Nil.Error())

		lens, err = rdb.Do(ctx, "JSON.ARRLEN", "a", "$.not_exists").Uint64Slice()
		require.NoError(t, err)
		require.EqualValues(t, []uint64{}, lens)
	})

	t.Run("JSON.ARRINSERT basics", func(t *testing.T) {
		arrInsertCmd := "JSON.ARRINSERT"
		require.NoError(t, rdb.Del(ctx, "a").Err())
		// key no exists
		require.EqualError(t, rdb.Do(ctx, arrInsertCmd, "not_exists", "$", 0, 1).Err(), redis.Nil.Error())
		// key not json
		require.NoError(t, rdb.Do(ctx, "SET", "no_json", "1").Err())
		require.Error(t, rdb.Do(ctx, arrInsertCmd, "no_json", "$", 0, 1).Err())
		// json path no exists
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":{}}`).Err())
		require.EqualValues(t, []interface{}{}, rdb.Do(ctx, arrInsertCmd, "a", "$.not_exists", 0, 1).Val())
		// json path not array
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, arrInsertCmd, "a", "$.a", 0, 1).Val())
		// index not a integer
		require.Error(t, rdb.Do(ctx, arrInsertCmd, "a", "$", "no", 1).Err())
		require.Error(t, rdb.Do(ctx, arrInsertCmd, "a", "$", 1.1, 1).Err())
		// args size < 4
		require.Error(t, rdb.Do(ctx, arrInsertCmd, "a", "$", 0).Err())
		// json path has one array
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":[1,2,3], "b":{"a":[4,5,6,7],"c":2},"c":[1,2,3,4],"e":[6,7,8],"f":{"a":[10,11,12,13,14], "g":2}}`).Err())
		require.EqualValues(t, []interface{}{int64(4)}, rdb.Do(ctx, arrInsertCmd, "a", "$.e", 1, 90).Val())
		require.Equal(t, "[{\"a\":[1,2,3],\"b\":{\"a\":[4,5,6,7],\"c\":2},\"c\":[1,2,3,4],\"e\":[6,90,7,8],\"f\":{\"a\":[10,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// insert many value
		require.EqualValues(t, []interface{}{int64(8)}, rdb.Do(ctx, arrInsertCmd, "a", "$.e", 2, 80, 81, 82, 83).Val())
		require.Equal(t, "[{\"a\":[1,2,3],\"b\":{\"a\":[4,5,6,7],\"c\":2},\"c\":[1,2,3,4],\"e\":[6,90,80,81,82,83,7,8],\"f\":{\"a\":[10,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// json path has many array
		require.EqualValues(t, []interface{}{int64(6), int64(5), int64(4)}, rdb.Do(ctx, arrInsertCmd, "a", "$..a", 1, 91).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[1,2,3,4],\"e\":[6,90,80,81,82,83,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// json path has many array and one is not array
		require.EqualValues(t, []interface{}{int64(5), nil}, rdb.Do(ctx, arrInsertCmd, "a", "$..c", 0, 92).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[92,1,2,3,4],\"e\":[6,90,80,81,82,83,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// index = 0
		require.EqualValues(t, []interface{}{int64(9)}, rdb.Do(ctx, arrInsertCmd, "a", "$.e", 0, 93).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[92,1,2,3,4],\"e\":[93,6,90,80,81,82,83,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// index < 0
		require.EqualValues(t, []interface{}{int64(10)}, rdb.Do(ctx, arrInsertCmd, "a", "$.e", -2, 94).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[92,1,2,3,4],\"e\":[93,6,90,80,81,82,83,94,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// index >= len
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, arrInsertCmd, "a", "$.e", 15, 95).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[92,1,2,3,4],\"e\":[93,6,90,80,81,82,83,94,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// index + len < 0
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, arrInsertCmd, "a", "$", -15, 96).Val())
		require.Equal(t, "[{\"a\":[1,91,2,3],\"b\":{\"a\":[4,91,5,6,7],\"c\":2},\"c\":[92,1,2,3,4],\"e\":[93,6,90,80,81,82,83,94,7,8],\"f\":{\"a\":[10,91,11,12,13,14],\"g\":2}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())

	})

	t.Run("JSON.OBJKEYS basics", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "a").Err())
		// key no exists
		require.EqualError(t, rdb.Do(ctx, "JSON.OBJKEYS", "not_exists", "$").Err(), redis.Nil.Error())
		// key not json
		require.NoError(t, rdb.Do(ctx, "SET", "no_json", "1").Err())
		require.Error(t, rdb.Do(ctx, "JSON.OBJKEYS", "no_json", "$").Err())
		// json path no exists
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a1":[1,2]}`).Err())
		require.EqualValues(t, []interface{}{}, rdb.Do(ctx, "JSON.OBJKEYS", "a", "$.not_exists").Val())
		// json path not object
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.OBJKEYS", "a", "$.a1").Val())
		// default path
		require.EqualValues(t, []interface{}{[]interface{}{"a1"}}, rdb.Do(ctx, "JSON.OBJKEYS", "a").Val())
		// json path has one object
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a1":{"b":1,"c":1}}`).Err())
		require.EqualValues(t, []interface{}{[]interface{}{"b", "c"}}, rdb.Do(ctx, "JSON.OBJKEYS", "a", "$.a1").Val())
		// json path has many object
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":{"a1":{"b":1,"c":1}},"b":{"a1":{"e":1,"f":1}}}`).Err())
		require.EqualValues(t, []interface{}{[]interface{}{"b", "c"}, []interface{}{"e", "f"}}, rdb.Do(ctx, "JSON.OBJKEYS", "a", "$..a1").Val())
		// json path has many object and one is not object
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":{"a1":{"b":1,"c":1}},"b":{"a1":[1]},"c":{"a1":{"e":1,"f":1}}}`).Err())
		require.EqualValues(t, []interface{}{[]interface{}{"b", "c"}, interface{}(nil), []interface{}{"e", "f"}}, rdb.Do(ctx, "JSON.OBJKEYS", "a", "$..a1").Val())
	})

	t.Run("JSON.ARRPOP basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[3,"str",2.1,{},[5,6]]`).Err())
		require.EqualValues(t, []interface{}{"[5,6]"}, rdb.Do(ctx, "JSON.ARRPOP", "a").Val())
		require.EqualValues(t, []interface{}{"2.1"}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "-2").Val())
		require.EqualValues(t, []interface{}{"{}"}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "3").Val())
		require.EqualValues(t, []interface{}{`"str"`}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "1").Val())
		require.EqualValues(t, []interface{}{"3"}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "0").Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "o", "$", `{"o":{"x":1},"s":"str","i":1,"d":2.2}`).Err())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRPOP", "o", "$.o", 1).Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRPOP", "o", "$.s", -1).Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRPOP", "o", "$.i", 0).Val())
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRPOP", "o", "$.d", 2).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[[0,1],[3,{"x":2.0}],"str",[4,[5,"6"]]]`).Err())
		require.EqualValues(t, []interface{}{`[5,"6"]`, nil, `{"x":2.0}`, "1"}, rdb.Do(ctx, "JSON.ARRPOP", "a", "$.*").Val())

		require.ErrorContains(t, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "str").Err(), "not started as an integer")
		require.ErrorContains(t, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "2str").Err(), "encounter non-integer characters")
		require.ErrorContains(t, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "-1str").Err(), "encounter non-integer characters")
		require.ErrorContains(t, rdb.Do(ctx, "JSON.ARRPOP", "a", "$", "0", "1").Err(), "wrong number of arguments")
	})

	t.Run("JSON.ARRTRIM basics", func(t *testing.T) {
		require.NoError(t, rdb.Del(ctx, "a").Err())
		// key no exists
		require.EqualError(t, rdb.Do(ctx, "JSON.ARRTRIM", "not_exists", "$", 0, 0).Err(), redis.Nil.Error())
		// key not json
		require.NoError(t, rdb.Do(ctx, "SET", "no_json", "1").Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "no_json", "$", 0, 0).Err())
		// json path no exists
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a1":{}}`).Err())
		require.EqualValues(t, []interface{}{}, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.not_exists", 0, 0).Val())
		// json path not array
		require.EqualValues(t, []interface{}{nil}, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 0, 0).Val())
		// json path has one array
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a1":[1,2,3,4,5,6,7,8,9]}`).Err())
		require.EqualValues(t, []interface{}{int64(5)}, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 2, 6).Val())
		// json path has many array
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":{"a1":[1,2,3,4,5,6]},"b":{"a1":["a",{},"b"]},"c":{"a1":[7,8,9,10,11]}}`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(3), int64(2), int64(3)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$..a1", 1, 3).Val())
		require.EqualValues(t, "[{\"a\":{\"a1\":[2,3,4]},\"b\":{\"a1\":[{},\"b\"]},\"c\":{\"a1\":[8,9,10]}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// json path has many array and one is not array
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"a":{"a1":[1,2,3,4,5,6]},"b":{"a1":{"b":1,"c":1}},"c":{"a1":[7,8,9,10]}}`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(3), interface{}(nil), int64(3)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$..a1", 1, 3).Val())
		require.EqualValues(t, "[{\"a\":{\"a1\":[2,3,4]},\"b\":{\"a1\":{\"b\":1,\"c\":1}},\"c\":{\"a1\":[8,9,10]}}]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start not a integer
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", "no", 1).Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 1.1, 1).Err())
		// stop not a integer
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 1, 1.1).Err())
		// args size != 5
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 0).Err())
		require.Error(t, rdb.Do(ctx, "JSON.ARRTRIM", "a", "$.a1", 0, 2, 3).Err())
	})

	t.Run("JSON.ARRTRIM special <start> and <stop> args", func(t *testing.T) {
		// start < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(4)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -5, 8).Val())
		require.EqualValues(t, "[[6,7,8,9]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start + len < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(6)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -20, 5).Val())
		require.EqualValues(t, "[[1,2,3,4,5,6]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start > len
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(0)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 15, 25).Val())
		require.EqualValues(t, "[[]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start = 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(9)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 0, 8).Val())
		require.EqualValues(t, "[[1,2,3,4,5,6,7,8,9]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// stop = 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(1)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -12, 0).Val())
		require.EqualValues(t, "[[1]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// stop < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(9)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 0, -2).Val())
		require.EqualValues(t, "[[1,2,3,4,5,6,7,8,9]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// len + stop < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(1)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 0, -20).Val())
		require.EqualValues(t, "[[1]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// stop > len
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(10)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 0, 20).Val())
		require.EqualValues(t, "[[1,2,3,4,5,6,7,8,9,10]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start > stop
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(0)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", 8, 5).Val())
		require.EqualValues(t, "[[]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start < 0 and stop < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(4)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -8, -5).Val())
		require.EqualValues(t, "[[3,4,5,6]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start < 0 , stop < 0 and start > end
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(0)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -5, -8).Val())
		require.EqualValues(t, "[[]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
		// start + len < 0 , stop + len < 0
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[1,2,3,4,5,6,7,8,9,10]`).Err())
		require.EqualValues(t, []interface{}([]interface{}{int64(1)}), rdb.Do(ctx, "JSON.ARRTRIM", "a", "$", -30, -20).Val())
		require.EqualValues(t, "[[1]]", rdb.Do(ctx, "JSON.GET", "a", "$").Val())
	})

	t.Run("JSON.TOGGLE basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `true`).Err())
		require.EqualValues(t, []interface{}{int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$").Val())
		require.Equal(t, `false`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"bool":true}`).Err())
		require.EqualValues(t, []interface{}{int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$.bool").Val())
		require.Equal(t, `{"bool":false}`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"bool":true,"bools":{"bool":true}}`).Err())
		require.EqualValues(t, []interface{}{int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$.bool").Val())
		require.Equal(t, `{"bool":false,"bools":{"bool":true}}`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"bool":true,"bools":{"bool":true}}`).Err())
		require.EqualValues(t, []interface{}{int64(0), int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$..bool").Val())
		require.Equal(t, `{"bool":false,"bools":{"bool":false}}`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"bool":false,"bools":{"bool":true}}`).Err())
		require.EqualValues(t, []interface{}{int64(1), int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$..bool").Val())
		require.Equal(t, `{"bool":true,"bools":{"bool":false}}`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"incorrectbool":99,"bools":{"bool":true},"bool":{"bool":false}}`).Err())
		require.EqualValues(t, []interface{}{nil, int64(1), int64(0)}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$..bool").Val())
		require.Equal(t, `{"bool":{"bool":true},"bools":{"bool":false},"incorrectbool":99}`, rdb.Do(ctx, "JSON.GET", "a").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `[99,true,99]`).Err())
		require.EqualValues(t, []interface{}{nil, int64(0), nil}, rdb.Do(ctx, "JSON.TOGGLE", "a", "$..*").Val())
		require.Equal(t, `[99,false,99]`, rdb.Do(ctx, "JSON.GET", "a").Val())
	})

	t.Run("JSON.ARRINDEX basics", func(t *testing.T) {
		arrIndexCmd := "JSON.ARRINDEX"
		require.NoError(t, rdb.Do(ctx, "SET", "a", `1`).Err())
		require.Error(t, rdb.Do(ctx, arrIndexCmd, "a", "$", `1`).Err())
		require.NoError(t, rdb.Do(ctx, "DEL", "a").Err())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", ` {"x":1, "y": {"x":1} } `).Err())
		require.Equal(t, []interface{}{}, rdb.Do(ctx, arrIndexCmd, "a", "$..k", `1`).Val())
		require.Error(t, rdb.Do(ctx, arrIndexCmd, "a", "$").Err())
		require.Error(t, rdb.Do(ctx, arrIndexCmd, "a", "$", ` 1, 2, 3`).Err())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"arr":[0,1,2,3,2,1,0]}`).Err())
		require.Equal(t, []interface{}{int64(0)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`).Val())
		require.Equal(t, []interface{}{int64(3)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `3`).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `4`).Val())
		require.Equal(t, []interface{}{int64(6)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, 1).Val())
		require.Equal(t, []interface{}{int64(6)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, -1).Val())
		require.Equal(t, []interface{}{int64(6)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, 6).Val())
		require.Equal(t, []interface{}{int64(6)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, 4, -0).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, 5, -1).Val())
		require.Equal(t, []interface{}{int64(6)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `0`, 5, 0).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `2`, -2, 6).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `"foo"`).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"arr":[0,1,2,3,4,2,1,0]}`).Err())

		require.Equal(t, []interface{}{int64(3)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `3`).Val())
		require.Equal(t, []interface{}{int64(5)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `2`, 3).Val())
		require.Equal(t, []interface{}{int64(1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `1`).Val())
		require.Equal(t, []interface{}{int64(2)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `2`, 1, 4).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `6`).Val())
		require.Equal(t, []interface{}{int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr", `3`, 0, 2).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"arr":[0,1,2]}`).Err())
		require.Equal(t, []interface{}{nil, nil, nil}, rdb.Do(ctx, arrIndexCmd, "a", "$.arr.*", `1`).Val())
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a1", "$", `{"arr":[[1],[2],[3]]}`).Err())
		require.Equal(t, []interface{}{int64(0), int64(-1), int64(-1)}, rdb.Do(ctx, arrIndexCmd, "a1", "$.arr.*", `1`).Val())
	})

	t.Run("JSON.NUMOP basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{ "foo": 0, "bar": "baz" }`).Err())
		require.Equal(t, `[1]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.foo", 1).Val())
		require.Equal(t, `[1]`, rdb.Do(ctx, "JSON.GET", "a", "$.foo").Val())
		require.Equal(t, `[3]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.foo", 2).Val())
		require.Equal(t, `[3.5]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.foo", 0.5).Val())

		// wrong type
		require.Equal(t, `[null]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.bar", 1).Val())

		require.Equal(t, `[]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.fuzz", 1).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `0`).Err())
		require.Equal(t, `[1]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$", 1).Val())
		require.Equal(t, `[2.5]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$", 1.5).Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"foo":0,"bar":42}`).Err())
		require.Equal(t, `[1]`, rdb.Do(ctx, "JSON.NUMINCRBY", "a", "$.foo", 1).Val())
		require.Equal(t, `[84]`, rdb.Do(ctx, "JSON.NUMMULTBY", "a", "$.bar", 2).Val())

		// overflow case
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "big_num", "$", "1.6350000000001313e+308").Err())
		require.Equal(t, `[1.6350000000001313e+308]`,
			rdb.Do(ctx, "JSON.NUMINCRBY", "big_num", "$", 1).Val())

		require.Error(t, rdb.Do(ctx, "JSON.NUMMULTBY", "big_num", "$", 2).Err())

		require.Equal(t, `1.6350000000001313e+308`, rdb.Do(ctx, "JSON.GET", "big_num").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "nested_obj_big_num", "$",
			`{"l1":{"l2_a":1.6350000000001313e+308,"l2_b":2}}`).Err())

		require.Equal(t, `[1.6350000000001313e+308]`,
			rdb.Do(ctx, "JSON.NUMINCRBY", "nested_obj_big_num", "$.l1.l2_a", 1).Val())

		require.Error(t, rdb.Do(ctx, "JSON.NUMMULTBY", "nested_obj_big_num", "$.l1.l2_a", 2).Err())

		require.Equal(t, `{"l1":{"l2_a":1.6350000000001313e+308,"l2_b":2}}`,
			rdb.Do(ctx, "JSON.GET", "nested_obj_big_num").Val())

		require.NoError(t, rdb.Do(ctx, "JSON.SET", "nested_arr_big_num", "$",
			`{"l1":{"l2":[0,1.6350000000001313e+308]}}`).Err())

		require.Error(t, rdb.Do(ctx, "JSON.NUMINCRBY", "nested_arr_big_num", "$.l1.l2[1]",
			`1.6350000000001313e+308`).Err())
		require.Error(t, rdb.Do(ctx, "JSON.NUMMULTBY", "nested_arr_big_num", "$.l1.l2[1]", 2).Err())

		require.Equal(t, `{"l1":{"l2":[0,1.6350000000001313e+308]}}`,
			rdb.Do(ctx, "JSON.GET", "nested_arr_big_num").Val())
	})

	t.Run("JSON.OBJLEN basics", func(t *testing.T) {
		require.NoError(t, rdb.Do(ctx, "JSON.SET", "a", "$", `{"x":[3], "nested": {"x": {"y":2, "z": 1}}}`).Err())
		vals, err := rdb.Do(ctx, "JSON.OBJLEN", "a", "$..x").Slice()
		require.NoError(t, err)
		require.EqualValues(t, 2, len(vals))
		// the first `x` path is not an object, so it should return nil
		require.EqualValues(t, nil, vals[0])
		// the second `x` path is an object with two fields, so it should return 2
		require.EqualValues(t, 2, vals[1])

		vals, err = rdb.Do(ctx, "JSON.OBJLEN", "a", "$.nested").Slice()
		require.NoError(t, err)
		require.EqualValues(t, 1, len(vals))
		require.EqualValues(t, 1, vals[0])

		vals, err = rdb.Do(ctx, "JSON.OBJLEN", "a", "$").Slice()
		require.NoError(t, err)
		require.EqualValues(t, 1, len(vals))
		require.EqualValues(t, 2, vals[0])

		vals, err = rdb.Do(ctx, "JSON.OBJLEN", "a", "$.no_exists_path").Slice()
		require.NoError(t, err)
		require.EqualValues(t, 0, len(vals))

		err = rdb.Do(ctx, "JSON.OBJLEN", "no-such-json-key", "$").Err()
		require.EqualError(t, err, redis.Nil.Error())
	})
}
