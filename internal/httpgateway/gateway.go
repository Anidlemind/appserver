package httpgateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	pb "carstats/proto"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	handler http.Handler
	conn    *grpc.ClientConn
}

func New(ctx context.Context, grpcAddr string) (*Server, error) {
	conn, err := grpc.DialContext(
		ctx,
		dialableAddr(grpcAddr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	authClient := pb.NewAuthClient(conn)
	statsClient := pb.NewStatsServiceClient(conn)

	mux := runtime.NewServeMux()

	if err := mux.HandlePath("POST", "/v1/auth/register", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		forwardJSONRPC(w, r, func(ctx context.Context) (proto.Message, error) {
			var req pb.RegisterRequest
			if err := decodeJSONBody(r, &req); err != nil {
				return nil, err
			}
			return authClient.Register(ctx, &req)
		})
	}); err != nil {
		conn.Close()
		return nil, err
	}

	if err := mux.HandlePath("POST", "/v1/auth/login", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		forwardJSONRPC(w, r, func(ctx context.Context) (proto.Message, error) {
			var req pb.LoginRequest
			if err := decodeJSONBody(r, &req); err != nil {
				return nil, err
			}
			return authClient.Login(ctx, &req)
		})
	}); err != nil {
		conn.Close()
		return nil, err
	}

	if err := mux.HandlePath("POST", "/v1/stats", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		forwardJSONRPC(w, r, func(ctx context.Context) (proto.Message, error) {
			var req pb.SubmitUserStatsRequest
			if err := decodeJSONBody(r, &req); err != nil {
				return nil, err
			}
			return statsClient.SubmitUserStats(ctxWithAuthHeader(ctx, r), &req)
		})
	}); err != nil {
		conn.Close()
		return nil, err
	}

	if err := mux.HandlePath("GET", "/v1/stats/{vehicle_type}", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		forwardJSONRPC(w, r, func(ctx context.Context) (proto.Message, error) {
			return statsClient.GetGlobalStats(ctxWithAuthHeader(ctx, r), &pb.GetGlobalStatsRequest{
				VehicleType: pathParams["vehicle_type"],
			})
		})
	}); err != nil {
		conn.Close()
		return nil, err
	}

	rootMux := http.NewServeMux()
	rootMux.Handle("/v1/", mux)
	rootMux.HandleFunc("/openapi.json", serveOpenAPI)
	rootMux.HandleFunc("/swagger", redirectSwagger)
	rootMux.HandleFunc("/swagger/", serveSwaggerUI)

	return &Server{
		handler: rootMux,
		conn:    conn,
	}, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func forwardJSONRPC(w http.ResponseWriter, r *http.Request, call func(context.Context) (proto.Message, error)) {
	resp, err := call(r.Context())
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	body, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func decodeJSONBody(r *http.Request, dst proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return status.Error(codes.InvalidArgument, "cannot read request body")
	}

	if err := protojson.Unmarshal(body, dst); err != nil {
		return status.Error(codes.InvalidArgument, "invalid JSON body")
	}

	return nil
}

func ctxWithAuthHeader(ctx context.Context, r *http.Request) context.Context {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader))
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(runtime.HTTPStatusFromCode(st.Code()))
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    st.Code().String(),
		"message": st.Message(),
	})
}

func dialableAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	return addr
}

func redirectSwagger(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
}

func serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/swagger/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}

func serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(openAPISpec))
}

const swaggerHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Carstats Swagger UI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
    <style>
      body { margin: 0; background: #f5f7fb; }
      .topbar { display: none; }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        persistAuthorization: true
      });
    </script>
  </body>
</html>`

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Carstats API",
    "version": "1.0.0",
    "description": "HTTP gateway over gRPC services for auth and vehicle statistics."
  },
  "servers": [
    { "url": "http://localhost:8080" }
  ],
  "components": {
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "token"
      }
    },
    "schemas": {
      "RegisterRequest": {
        "type": "object",
        "required": ["email", "password"],
        "properties": {
          "email": { "type": "string", "example": "user@example.com" },
          "password": { "type": "string", "example": "secret123" }
        }
      },
      "RegisterResponse": {
        "type": "object",
        "properties": {
          "user_id": { "type": "integer", "format": "int64", "example": 1 },
          "token": { "type": "string", "example": "token" }
        }
      },
      "LoginRequest": {
        "type": "object",
        "required": ["email", "password"],
        "properties": {
          "email": { "type": "string", "example": "user@example.com" },
          "password": { "type": "string", "example": "secret123" }
        }
      },
      "LoginResponse": {
        "type": "object",
        "properties": {
          "token": { "type": "string", "example": "token" }
        }
      },
      "SubmitUserStatsRequest": {
        "type": "object",
        "required": ["vehicle_type", "fuel_liters", "distance_km"],
        "properties": {
          "vehicle_type": { "type": "string", "example": "sedan" },
          "fuel_liters": { "type": "number", "format": "double", "example": 20.5 },
          "distance_km": { "type": "number", "format": "double", "example": 250 },
          "recorded_at": { "type": "string", "example": "2026-04-21T10:30:00Z" }
        }
      },
      "SubmitUserStatsResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean", "example": true },
          "message": { "type": "string", "example": "stat accepted into raw storage" }
        }
      },
      "GetGlobalStatsResponse": {
        "type": "object",
        "properties": {
          "vehicle_type": { "type": "string", "example": "sedan" },
          "total_fuel_liters": { "type": "number", "format": "double", "example": 30.5 },
          "total_distance_km": { "type": "number", "format": "double", "example": 350 },
          "total_records": { "type": "integer", "format": "int64", "example": 2 },
          "avg_fuel_per_km": { "type": "number", "format": "double", "example": 0.08714 }
        }
      },
      "Error": {
        "type": "object",
        "properties": {
          "code": { "type": "string", "example": "Unauthenticated" },
          "message": { "type": "string", "example": "authorization token is required" }
        }
      }
    }
  },
  "paths": {
    "/v1/auth/register": {
      "post": {
        "summary": "Register user",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/RegisterRequest" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Registered",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/RegisterResponse" }
              }
            }
          },
          "400": {
            "description": "Invalid request",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/v1/auth/login": {
      "post": {
        "summary": "Login user",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/LoginRequest" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Logged in",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/LoginResponse" }
              }
            }
          },
          "401": {
            "description": "Invalid credentials",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/v1/stats": {
      "post": {
        "summary": "Submit vehicle stats",
        "security": [{ "bearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/SubmitUserStatsRequest" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Accepted",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/SubmitUserStatsResponse" }
              }
            }
          },
          "401": {
            "description": "Missing or invalid token",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/v1/stats/{vehicle_type}": {
      "get": {
        "summary": "Get aggregated stats by vehicle type",
        "security": [{ "bearerAuth": [] }],
        "parameters": [
          {
            "name": "vehicle_type",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Stats",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/GetGlobalStatsResponse" }
              }
            }
          },
          "401": {
            "description": "Missing or invalid token",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    }
  }
}`
