Place the HDG formatter XML files in this directory before building the binary.

Files must be named after the configured language, for example:

- `deutsch_formatters.xml`
- `english_formatters.xml`

The exporter embeds this directory at build time and reads
`formatters/<HDG_LANGUAGE>_formatters.xml` from the binary at startup.
