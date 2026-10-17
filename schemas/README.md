# llm-wiki frontmatter schemas

These JSON schemas and Markdown scaffolds were emitted by the installed
`pwc-llm-wiki 0.5.0` `spaces create` command in an isolated temporary repository.
They are the engine's built-in schemas, retained with this wiki so registration,
validation and deployment use the same contract. Source:
https://github.com/pwc-ch-adv-ai/llm-wiki (MIT OR Apache-2.0).

They describe wiki page metadata, not json2pptx slide input. The engine requires
this directory alongside the repository-root `wiki.toml` when registering an
existing wiki. Automatic ingest commits are disabled in that configuration.
