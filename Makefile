# Directorio donde se guardan los binarios compilados
BIN_DIR := bin

# Ficheros proto a compilar (ajusta rutas según tu estructura)
PROTO_FILES := \
	internal/adapters/grpc/protos/manager/manager.proto \
	internal/adapters/grpc/protos/worker/worker.proto

# Directorio de salida para los .pb.go
PROTO_OUT := ./

# Docker Tags
MANAGER_IMAGE := devops-platform/manager:latest
WORKER_IMAGE := devops-platform/worker:latest

# Comandos de generación de Protobuf
generate:
	@echo "Generando código protobuf..."
	rm -f internal/adapters/grpc/protos/manager/*pb.go adapters/grpc/protos/worker/*pb.go
	protoc \
		--proto_path=. \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

# Instalar dependencias
deps:
	@echo "Instalando dependencias..."
	go mod tidy

# Validar formato del código y dependencias
lint: deps
	go fmt ./...           # Formato
	go vet ./...           # Detectar errores potenciales
	go mod tidy            # Limpia módulos no usados

# Compilar todos los módulos
build:
	rm -fr workerManager/$(BIN_DIR) worker/$(BIN_DIR)
	CGO_ENABLED=0 go build -o workerManager/$(BIN_DIR)/workerManager ./workerManager
	CGO_ENABLED=0 go build -o worker/$(BIN_DIR)/worker ./worker

# Construir las imágenes Docker
docker-build: build docker-build-manager docker-build-worker

docker-build-manager: build
	docker build -t $(MANAGER_IMAGE) -f workerManager/Dockerfile workerManager/$(BIN_DIR)

docker-build-worker:
	docker build -t $(WORKER_IMAGE) -f worker/Dockerfile worker/$(BIN_DIR)

# Ejecutar las pruebas unitarias y de integración
test:
	go test -v -count=1 ./test/...

# Pruebas específicas para integración continua (simulación de integración E2E con Docker)
ci-test: docker-build
	# Simulación de E2E usando imágenes recién creadas
	# Aquí podrías usar testcontainers-go o lanzar contenedores directamente para pruebas
	@echo "Ejecutar pruebas de integración continua"

# Limpieza
clean:
	rm -rf workerManager/$(BIN_DIR) worker/$(BIN_DIR)
	docker rmi -f $(MANAGER_IMAGE) $(WORKER_IMAGE) || true

# Alias
all: build test docker-build ci-test

.PHONY: generate build lint docker-build docker-build-manager docker-build-worker test ci-test clean all deps
