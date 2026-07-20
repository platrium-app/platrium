---
name: write-docs
description: Skill for writing developer documentation for Platrium. USE WHEN asked to write, update, or expand the docs.
---

# Writing Documentation for Platrium

You are documenting Platrium, a high-performance self-hosted file sync and sharing platform. When the user asks you to write or update documentation (located in `docs/content/docs/`), you MUST strictly adhere to the following stylistic and operational guidelines.

## Tone & Dialect: Casual, Inviting, and Exciting!
The absolute most important rule is the tone. Our documentation must be super friendly to new devs who might not be heavily invested in massive enterprise architectures (like TypeSpec, DAGs, Smithy, etc.).

- **DO NOT** use corporate "synergy" speak, dry textbook language, or robotic phrasing.
- **DO** write like you are excitedly explaining a cool project to a friend. Use exclamation points where appropriate, highlight cool features, and make the developer feel welcome.
- **DO NOT** be overly verbose. Keep paragraphs short and punchy.

## Explain the "Why", Not Just the "How"
Newcomers need to understand *why* Platrium is built the way it is to ensure long-term understanding of the codebase.
If the user asks you to document a feature or pipeline, you **MUST** actively reason about the code and prompt the user for design decisions.

**Example Agent Prompt:**
> "I can write the docs for the GraphQL implementation! Before I do, could you give me some context on *why* we chose GraphQL over standard REST for the UI? What specific pain points were you solving? I want to make sure I document the architectural reasoning for future contributors!"

If you lack context on a design decision, **ASK THE USER**. Do not hallucinate enterprise jargon. 

## Highly Visual Formatting
Platrium docs heavily utilize visual components to break up text. You must use them!

- **Fumadocs Accordions:** Use `<Accordion>` and `<Accordions>` components liberally for FAQs, Design Decisions, or deep-dives that aren't strictly necessary for the main flow.
- **Mermaid Diagrams:** Standard markdown code blocks with ` ```mermaid ` work out of the box. Use them to map out complex flows (like Nx DAG dependencies or cross-platform architectures) instead of writing a wall of text.
- **Fumadocs Twoslash:** If you are documenting code snippets (especially TypeScript/TypeSpec), use ` ```ts twoslash ` so the user gets rich interactive hovers!
- **Admonitions (Notes/Warnings):** DO NOT use GitHub alert syntax (`> [!NOTE]`). Instead, use Docusaurus/Fumadocs admonition directive syntax (e.g., `:::info`, `:::note`, `:::warning`) to highlight critical information.
- **No Numbered Headings:** NEVER use numbered prefixes for markdown headings (e.g., use `## The Generators` instead of `## 1. The Generators`). Numbers are not automatic and break easily when inserting new sections.
- **No H1 Page Titles:** NEVER start the page content with a `# Heading` matching the page title. Fumadocs automatically renders an H1 from the frontmatter `title` field. Starting the file with an H1 causes the title to be displayed twice. So, add a description and title as a part of the fumadocs page header.

## Specific Examples of Existing Style
When writing a new page, look at existing pages (like `docs/content/docs/architecture/build-pipeline.mdx` or `docs/content/docs/api-design/index.mdx`) for reference on the tone.

Notice how we explain the Nx monorepo:
> *"Makefiles are a rite of passage, but for a massive polyglot monorepo... they quickly turn into an unmaintainable "PITA" (Pain In The Asterisk)."*

This is the exact tone you should aim for!
