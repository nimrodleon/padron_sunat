# Padrón Reducido SUNAT 🇵🇪

Una herramienta en línea de comandos (CLI) escrita en Go para descargar automáticamente el **padrón reducido de
contribuyentes SUNAT** y convertirlo a una base de datos **SQLite**, lista para consultar o integrarse con sistemas
externos.

## 🚀 Características

- 📥 Descarga automática del padrón reducido SUNAT (`.zip`)
- 🧩 Descompresión y decodificación (ISO-8859-1)
- 💾 Conversión directa a SQLite (`.db`)
- 🧪 Compatible con sistemas Snap Linux (modo `strict`)
- 🌐 Acceso a red controlado para producción

## 📦 Instalación

### 🔧 Snap (recomendado)

```bash
sudo snap install padron-sunat
