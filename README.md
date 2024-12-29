# DevOps Platform

Sistema distribuido para la ejecución remota de comandos y gestión de procesos, implementado en Go utilizando gRPC para la comunicación entre componentes.

## Arquitectura

El sistema está compuesto por dos componentes principales:

### Worker Manager

Actúa como el orquestador central del sistema, responsable de:
- Gestionar el registro de workers
- Distribuir comandos a los workers disponibles
- Monitorizar el estado de los procesos
- Proporcionar una API gRPC para clientes externos

### Workers

Nodos de ejecución que:
- Se registran automáticamente con el Worker Manager
- Ejecutan comandos de forma aislada
- Transmiten la salida en tiempo real
- Reportan métricas y estado de salud

## Estructura del Proyecto

```
.
├── adapters/
│   └── grpc/
│       ├── client/       # Clientes gRPC
│       ├── protos/       # Definiciones Protobuf
│       └── server/       # Implementaciones de servidores gRPC
├── core/
│   ├── domain/          # Entidades y modelos de dominio
│   ├── interfaces/      # Interfaces del dominio
│   └── usecase/         # Casos de uso de la aplicación
├── worker/              # Implementación del Worker
├── workerManager/       # Implementación del Worker Manager
└── test/               # Tests de integración y E2E
```

## Características

- **Comunicación Bidireccional**: Streaming gRPC para transmisión de salida en tiempo real
- **Registro Automático**: Los workers se auto-registran con el manager
- **Tolerancia a Fallos**: Reintentos automáticos en las conexiones
- **Monitorización**: Healthchecks y métricas de los workers
- **Aislamiento**: Cada worker ejecuta comandos en su propio espacio
- **Escalabilidad**: Diseño distribuido que permite agregar workers dinámicamente

## Requisitos

- Go 1.22 o superior
- Protocol Buffers
- Docker (para contenedores y tests)

## Instalación

1. Clonar el repositorio:
```bash
git clone https://github.com/Rubentxu/devops-platform.git
cd devops-platform
```

2. Instalar dependencias:
```bash
make deps
```

3. Generar código protobuf:
```bash
make proto
```

4. Compilar:
```bash
make build
```

## Uso

### Iniciar Worker Manager:
```bash
make run-manager
```

### Iniciar Worker:
```bash
make run-worker
```

### Variables de Entorno

Worker:
- `MANAGER_HOST`: Host del Worker Manager (default: "localhost")
- `MANAGER_PORT`: Puerto del Worker Manager (default: "50051")

Worker Manager:
- No requiere configuración especial por defecto
- Escucha en el puerto 50051

## Docker

Construir imágenes:
```bash
make docker-build
```

Ejecutar con Docker Compose:
```bash
docker-compose up
```

## Tests

Ejecutar tests unitarios:
```bash
make test
```

Tests de integración:
```bash
make ci-test
```

## API gRPC

### Worker Registration Service
- `RegisterWorker`: Registra un nuevo worker en el sistema

### Process Management Service
- `ExecuteDistributedCommand`: Ejecuta un comando en un worker disponible
- `TerminateProcess`: Termina un proceso en ejecución
- `GetProcessStatus`: Consulta el estado de un proceso

### Worker Process Service
- `ExecuteCommand`: Ejecuta un comando y transmite su salida
- `TerminateProcess`: Termina un proceso en ejecución
- `HealthCheck`: Monitoriza el estado del worker
- `ReportMetrics`: Transmite métricas del worker

## Contribuir

1. Fork el repositorio
2. Crear una rama para la feature (`git checkout -b feature/amazing-feature`)
3. Commit los cambios (`git commit -m 'Add some amazing feature'`)
4. Push a la rama (`git push origin feature/amazing-feature`)
5. Abrir un Pull Request

## Licencia

Este proyecto está licenciado bajo la Licencia MIT - ver el archivo [LICENSE](LICENSE) para más detalles.

## Contacto

Rubén Túñez - [@rubentxu](https://twitter.com/rubentxu)

Link del proyecto: [https://github.com/Rubentxu/devops-platform](https://github.com/Rubentxu/devops-platform) 