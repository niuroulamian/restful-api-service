FROM golang:1-alpine AS development

ENV PROJECT_PATH=/service
ENV PATH=$PATH:$PROJECT_PATH/build
ENV CGO_ENABLED=1
ENV GO_EXTRA_BUILD_ARGS="-a -installsuffix cgo"

RUN apk add --no-cache ca-certificates make git bash protobuf alpine-sdk

RUN mkdir -p $PROJECT_PATH
COPY . $PROJECT_PATH
WORKDIR $PROJECT_PATH

# credentials to pull the dependencies from our private repos
ARG GO_DEPLOY_MACHINE=gitlab.com
ARG GO_DEPLOY_USER
ARG GO_DEPLOY_PASS
RUN echo "machine $GO_DEPLOY_MACHINE login $GO_DEPLOY_USER password $GO_DEPLOY_PASS" >/root/.netrc

RUN make dev-requirements clean build

FROM alpine:latest AS production

WORKDIR /root/
RUN apk --no-cache add ca-certificates
RUN mkdir /etc/service
COPY --from=development /service/build/ .
ENTRYPOINT ["./service"]
