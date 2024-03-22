/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkgio

var yamldoc1 = `
# comment
apiVersion: v1 #comment
kind: ConfigMap
data:
  foo: bar
  list:
  - item1
  - item2
`

var yamldoc2 = `
# comment
apiVersion: v1 #comment
kind: ConfigMap
metadata:
  annotations:
    a: b
data:
  foo: bar
  list:
  - item1
  - item2
`

var yamldoc3 = `
# comment
apiVersion: v1 #comment
kind: ConfigMap
metadata:
  annotations:
    a: b
data:
  foo: bar
  list:
  - item1
  - item2
---
# comment
apiVersion: v1 #comment
kind: ConfigMap
metadata:
  annotations:
    c: d
data:
foo: bar
list:
- item1
- item2
`
