//go:build deps
// +build deps

package internal

import (
	_ "github.com/gin-gonic/gin"
	_ "github.com/neo4j/neo4j-go-driver/v5/neo4j"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/redis/go-redis/v9"
	_ "github.com/segmentio/kafka-go"
	_ "github.com/milvus-io/milvus-sdk-go/v2/client"
	_ "github.com/opensearch-project/opensearch-go/v3/opensearchapi"
	_ "github.com/minio/minio-go/v7"
	_ "github.com/spf13/cobra"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/proto"
)
