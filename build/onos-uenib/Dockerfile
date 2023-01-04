# SPDX-FileCopyrightText: 2019-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

FROM onosproject/golang-build:v1.3.0 as build

ENV GO111MODULE=on
COPY . /go/src/github.com/onosproject/onos-uenib
RUN cd /go/src/github.com/onosproject/onos-uenib && GOFLAGS=-mod=vendor make build

FROM frolvlad/alpine-glibc:alpine-3.12

USER nobody

COPY --from=build /go/src/github.com/onosproject/onos-uenib/build/_output/onos-uenib /usr/local/bin/onos-uenib

ENTRYPOINT ["onos-uenib"]
