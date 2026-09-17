<p align="center">
  <img src="assets/brand/mnemo-logo.png" alt="logo de mnemo" width="128" height="128">
</p>

<h1 align="center">mnemo</h1>

<p align="center">
  <strong>Memoria persistente para agentes de programación.</strong>
</p>

<p align="center">
</p>

<p align="center">
  <a href="README.md">English</a> · <a href="README.es.md">Español</a> · <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go" src="https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo/releases"><img alt="Estado" src="https://img.shields.io/badge/status-alpha-orange"></a>
  <a href="https://sqlite.org"><img alt="Storage" src="https://img.shields.io/badge/storage-SQLite%2BFTS5-003B57?logo=sqlite&logoColor=white"></a>
  <a href="https://github.com/jmeiracorbal/mnemo"><img alt="Plataforma" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey"></a>
  <a href="LICENSE"><img alt="Licencia" src="https://img.shields.io/badge/license-Apache%202.0-blue"></a>
</p>

<p align="center">
  <a href="#inicio-rapido">Inicio rápido</a> ·
  <a href="#por-que-mnemo">Por qué mnemo</a> ·
  <a href="#agentes-soportados">Agentes</a> ·
  <a href="#documentacion">Documentación</a> ·
  <a href="ROADMAP.md">Roadmap</a>
</p>

---

## ¿Qué es mnemo?

mnemo es una capa de memoria local para desarrollo con agentes. Guarda decisiones, bugs, convenciones, descubrimientos y resúmenes de sesión en SQLite, y los expone mediante herramientas MCP o nativas, hooks y Agent Skills portables. Un controller local de eventos es el único escritor normal de SQLite: los agentes publican eventos durables y comandos de memoria en lugar de abrir la base de datos.

En lugar de repartir conocimiento entre `MEMORY.md`, memorias nativas del editor, transcripciones y notas humanas, mnemo ofrece a todos los agentes soportados una misma fuente de verdad por proyecto.

<a id="inicio-rapido"></a>

## Inicio rápido

Instala el binario y configura los agentes detectados:

```bash
curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh | MNEMO_VERSION=v1.0.0-alpha.1 bash
```

Esto fija la alpha actual. La instalación sin versión y `mnemo update` siguen las releases estables.

Activa mnemo en un proyecto:

```bash
cd tu-proyecto
mnemo init --agent=all
```

Comprueba que todo está conectado:

```bash
mnemo doctor --agent=all --path=.
```

En tu agente, usa `mem_save` para guardar una decisión y `mem_search` para recuperarla. La CLI también permite buscar memorias existentes:

```bash
mnemo search "SQLite" --project "$(mnemo json id < .mnemo)"
```

El controller se instala como servicio de usuario durante el setup. Consulta [Eventos durables](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events) para conocer las garantías de entrega y los errores.

<a id="por-que-mnemo"></a>

## Por qué mnemo

| Problema | mnemo aporta |
|---|---|
| Los agentes olvidan decisiones entre sesiones | Memoria persistente de proyecto en `~/.mnemo/memory.db` |
| Los hooks o plugins pueden perder escrituras o competir entre sí | Un controller local con entrega durable y transacciones SQLite idempotentes |
| Los archivos markdown de memoria se desordenan | Observaciones estructuradas, tags, topic keys y estados de revisión |
| Los hooks globales pueden ser peligrosos | Activación opt-in por proyecto mediante `.mnemo`; el resto se ignora |
| El setup puede fallar en silencio | `mnemo doctor` y `mnemo setup status` explican qué está configurado |
| Se acumulan proyectos o memorias duplicadas | Herramientas de merge de proyectos y curación de memoria |

## Funcionalidades

| Funcionalidad | Qué hace |
|---|---|
| **Activación por proyecto** | Los hooks globales solo actúan cuando existe una marca `.mnemo` válida. |
| **Herramientas MCP** | Los agentes pueden usar `mem_save`, `mem_search`, `mem_context`, `mem_current_project`, `mem_doctor` y más. |
| **Controller de eventos durables** | Un servicio JetStream por usuario es el único escritor normal de SQLite; los publicadores no abren el store. |
| **Sesiones del controller** | Los ID nativos de cada agente vinculan eventos y herramientas a una sesión canónica. |
| **Agent Skills portables** | Enseñan a los agentes compatibles cuándo y cómo usar mnemo sin recurrir a memoria nativa. |
| **Captura pasiva** | Las herramientas de memoria extraen aprendizajes del contenido proporcionado por el agente. |
| **Provenance de agentes** | Registra metadatos consultables en SQL sobre agente, origen, tool, modelo y cliente MCP en escrituras que los aportan. |
| **Diagnóstico** | `mnemo doctor` comprueba activación, setup global, MCP, hooks, memorias competidoras y salud de migraciones de la base de datos. |
| **Seguridad de base de datos** | Las migraciones seguras se aplican automáticamente; `mnemo db migrate --check` valida el store local para CI o troubleshooting. |
| **Autoactualización** | Los binarios publicados comprueban nuevas releases en uso interactivo y pueden confirmar, descargar e instalar con `mnemo update`. |
| **CLI programable** | La ayuda generada con Cobra mantiene alineados el menú de comandos y sus subcomandos con el ejecutable. |
| **Mantenimiento de proyectos** | `mnemo projects list`, `mnemo projects merge` y `mnemo projects rename` ayudan a depurar identidades duplicadas o poco claras. |
| **Curación de memoria** | `mnemo memories review` detecta observaciones duplicadas o conflictivas para reparación aprobada. |

## Agentes soportados

<p align="center">
  <img alt="Claude Code" src="https://img.shields.io/badge/Claude%20Code-soportado-6B46C1?logo=claudecode&logoColor=white">
  <img alt="Codex" src="https://img.shields.io/badge/Codex-soportado-00A67E?logo=openai&logoColor=white">
  <img alt="Cursor" src="https://img.shields.io/badge/Cursor-soportado-111111?logo=cursor&logoColor=white">
  <img alt="OpenCode" src="https://img.shields.io/badge/OpenCode-soportado-F97316?logo=opencode&logoColor=white">
  <img alt="Pi" src="https://img.shields.io/badge/Pi-soportado-0EA5E9">
</p>

| Agente | MCP | Hooks / runtime | Instrucciones globales | Skills | Estado |
|---|---:|---:|---:|---:|---|
| Claude Code | Sí | Hooks del plugin o setup del instalador | Sí | Sí | Soportado |
| Codex | Sí | Hooks de sesión | Sí | Sí | Soportado |
| Cursor | Sí | Hook de prompt | Sí | Sí | Soportado |
| OpenCode | Sí | Eventos del plugin | Sí | Sí | Soportado |
| Pi | Herramientas nativas | Extensión nativa | Sí | Sí | Soportado |

El setup global se instala una vez. La activación del proyecto sigue siendo local y explícita:

```text
project/
├── .mnemo      # ID de proyecto + agentes activados, ignorado por git
├── AGENTS.md   # autoridad de memoria compartida
├── CLAUDE.md   # reglas específicas de Claude cuando se selecciona
├── .cursor/    # reglas de Cursor cuando se selecciona
└── .pi/        # extensiones de prompt de Pi cuando se selecciona
```

## Verlo en acción

```text
$ mnemo doctor --agent=all --path=.
status: ok
checks: project marker, binary, MCP, hooks, instructions, store

$ mnemo context miapp
## Memoria de sesiones anteriores
- Se eligió SQLite FTS5 para búsqueda local.
- Los hooks refrescados deben conservar permisos de ejecución.

$ mnemo memories review --project=miapp
No potential memory conflicts found.
```

## Opciones de instalación

| Vía | Cuándo usarla | Comando |
|---|---|---|
| Alpha actual | Quieres probar `v1.0.0-alpha.1` | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; MNEMO_VERSION=v1.0.0-alpha.1 bash</code> |
| Última estable | Quieres binario estable + setup de agentes detectados | <code>curl -sSf https://raw.githubusercontent.com/jmeiracorbal/mnemo/main/install.sh &#124; bash</code> |
| Agente explícito | Solo quieres una integración | `bash -s -- --agent=codex` |
| Todos los agentes | Quieres preparar todas las integraciones | `bash -s -- --agent=all` |
| Plugin de Claude | Usas el marketplace de Claude Code | `claude plugin install mnemo@mnemo` |
| Build desde fuente | Desarrollas mnemo | `go build -o ~/.local/bin/mnemo ./cmd/mnemo/` |

Consulta la [guía de instalación](https://github.com/jmeiracorbal/mnemo/wiki/Installation).

### Actualizaciones

Los binarios publicados de mnemo comprueban GitHub Releases durante el uso CLI
interactivo. Si existe una release más nueva, mnemo muestra las versiones
instalada/disponible y pregunta antes de modificar nada:

```bash
mnemo update
mnemo update --yes --agent=all
mnemo update --check --json
```

`mnemo update` descarga el instalador oficial, lo fija a la última release
detectada y refresca los archivos de integración de mnemo para agentes después
de instalar. No actualiza Claude Code, Codex, Cursor ni otras aplicaciones de
agente. Reinicia las sesiones activas de agentes después de actualizar para que
recarguen el binario, hooks y skills refrescados. Las comprobaciones se omiten
en rutas MCP, hooks y salidas JSON para no romper integraciones machine-readable.

<a id="documentacion"></a>

## Documentación

| Guía | Contenido |
|---|---|
| [Wiki](https://github.com/jmeiracorbal/mnemo/wiki) | Documentación de usuario y navegación. |
| [Instalación](https://github.com/jmeiracorbal/mnemo/wiki/Installation) | Binario, servicio controller, activación y verificación. |
| [Integración con agentes](https://github.com/jmeiracorbal/mnemo/wiki/Agent-Integrations) | ID nativos, hooks, herramientas y marca `.mnemo`. |
| [Eventos durables](https://github.com/jmeiracorbal/mnemo/wiki/Durable-Events) | Arquitectura, entrega y errores del controller. |
| [Referencia CLI](https://github.com/jmeiracorbal/mnemo/wiki/CLI-Reference) | Comandos actuales y herramientas MCP. |
| [Diagnóstico](https://github.com/jmeiracorbal/mnemo/wiki/Troubleshooting) | Comprobaciones y recuperación del controller. |
| [Almacenamiento](https://github.com/jmeiracorbal/mnemo/wiki/Storage-and-Migrations) | SQLite, migraciones y sqlc. |
| [Roadmap](ROADMAP.md) | Trabajo planificado de producto y mantenimiento. |

## Principios de diseño

- **Local-first:** la memoria permanece en tu máquina en SQLite.
- **Neutral entre agentes:** una sola autoridad de memoria para todos los agentes soportados.
- **Opt-in por proyecto:** las integraciones globales no actúan sin `.mnemo`.
- **Diagnosticable:** cada superficie de setup puede comprobarse sin modificar nada.
- **Reparable:** duplicados de proyecto y conflictos de memoria son visibles y corregibles desde CLI.

## Licencia

[Apache 2.0](LICENSE): puedes usar, modificar y distribuir libremente, conservando el aviso de copyright e incluyendo [NOTICE](NOTICE) en las distribuciones.
