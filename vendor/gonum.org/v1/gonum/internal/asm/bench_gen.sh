#!/usr/bin/env bash

# Copyright ©2016 The Gonum Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

cat c64/bench_test.go \
    | gofmt -r 'complex(float32(n), float32(n)) -> float32(n)' \
    | gofmt -r 'complex64 -> float32' \
    | gofmt -r '1 + 1i -> 1' \
    | gofmt -r '2 + 2i -> 2' \
    | sed 's/C64/F32/g' \
    | sed 's/c64/f32/g' \
    > f32/bench_test.go

cat c64/bench_test.go \
    | gofmt -r 'complex(float32(n), float32(n)) -> float64(n)' \
    | gofmt -r 'complex64 -> float64' \
    | gofmt -r '1 + 1i -> 1' \
    | gofmt -r '2 + 2i -> 2' \
    | sed 's/C64/F64/g' \
    | sed 's/c64/f64/g' \
    > f64/bench_test.go

cat c64/bench_test.go \
    | gofmt -r 'float32 -> float64' \
    | gofmt -r 'complex64 -> complex128' \
    | sed 's/C64/C128/g' \
    | sed 's/c64/c128/g' \
    > c128/bench_test.go
