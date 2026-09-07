# Build the paper chapters into a single PDF.

.DEFAULT_GOAL := help

TARGET_DIR := rendered
BUILD_DIR  := $(TARGET_DIR)/.build

# Read version and watermark settings from metadata.yaml
VERSION := $(shell sed -n 's/^version:[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}[[:space:]]*$$/\1/p' metadata.yaml)
BUILD_DATE := $(shell date +%F)
DRAFT_WATERMARK := $(shell sed -n 's/^draft_watermark:[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}[[:space:]]*$$/\1/p' metadata.yaml | tr '[:upper:]' '[:lower:]')
WATERMARK_TEXT_RAW := $(shell sed -n 's/^watermark_text:[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}[[:space:]]*$$/\1/p' metadata.yaml)
WATERMARK_TEXT := $(if $(strip $(WATERMARK_TEXT_RAW)),$(strip $(WATERMARK_TEXT_RAW)),DRAFT)

OUTPUT := $(TARGET_DIR)/the-metasystem-$(VERSION).pdf

# index.md is the repository's navigation page. Its opening sections are the
# paper's preface, so the build reuses them and drops the chapter link list,
# which a PDF gets from its table of contents instead. Set PREFACE= to omit it.
PREFACE := $(BUILD_DIR)/00-preface.md

CHAPTERS := \
	01-the-shift.md \
	02-back-to-intent.md \
	03-the-new-workforce.md \
	04-the-mimicry-trap.md \
	05-first-principles.md \
	06-proof-over-trust.md \
	07-roles-from-first-principles.md \
	08-memory-and-coordination.md \
	09-the-living-system.md \
	10-care.md \
	11-economy.md \
	12-learning-systems.md \
	13-the-human-role.md \
	14-the-sitting.md \
	15-the-transition.md \
	16-self-application.md \
	17-outlook.md \
	18-appendix-functional-design.md

SOURCES := metadata.yaml $(PREFACE) $(CHAPTERS)

# The chapters carry their own numbers in their headings ("# 1. The Shift"),
# so --number-sections is deliberately absent: LaTeX would number them again.
PANDOC_OPTIONS := --toc --top-level-division=chapter --from=gfm+yaml_metadata_block

# Mermaid diagrams in the appendix are rendered to images when a renderer is
# available; without one the build still succeeds and leaves the diagram source
# visible. Point MMDC at a mermaid-cli binary, or run `make mermaid-cli`.
MMDC ?= $(firstword \
	$(wildcard $(BUILD_DIR)/node_modules/.bin/mmdc) \
	$(shell command -v mmdc 2>/dev/null))
MERMAID_FILTER := mermaid.lua
MERMAID_CONFIG := mermaid-config.json
MERMAID_LAYOUT := mermaid-layout.json
TABLE_FILTER := tables.lua
SCENARIO_FILTER := scenario.lua
TAGLINE_FILTER := tagline.lua
STYLE := style.tex
FILTERS := $(MERMAID_FILTER) $(TABLE_FILTER) $(SCENARIO_FILTER) $(TAGLINE_FILTER)

# MacTeX installs xelatex outside PATH; tectonic is a self-contained fallback.
PDF_ENGINE ?= $(firstword \
	$(wildcard /Library/TeX/texbin/xelatex) \
	$(shell command -v xelatex 2>/dev/null) \
	$(shell command -v tectonic 2>/dev/null))

HEADER_INCLUDES_BASE := \usepackage{fancyhdr} \pagestyle{fancy} \fancyhf{} \fancyfoot[L]{$(VERSION) $(BUILD_DATE)} \fancyfoot[C]{Wido Riezebos} \fancyfoot[R]{\thepage} \renewcommand{\headrulewidth}{0pt} \renewcommand{\footrulewidth}{0.4pt} \fancypagestyle{plain}{\fancyhf{} \fancyfoot[L]{$(VERSION) $(BUILD_DATE)} \fancyfoot[C]{Wido Riezebos} \fancyfoot[R]{\thepage} \renewcommand{\headrulewidth}{0pt} \renewcommand{\footrulewidth}{0.4pt}}

ifeq ($(DRAFT_WATERMARK),true)
HEADER_INCLUDES_WATERMARK := \usepackage{draftwatermark} \SetWatermarkText{$(WATERMARK_TEXT)} \SetWatermarkScale{1.8} \SetWatermarkLightness{0.9}
else
HEADER_INCLUDES_WATERMARK :=
endif

HEADER_INCLUDES := $(HEADER_INCLUDES_BASE) $(HEADER_INCLUDES_WATERMARK)

PANDOC_PDF_OPTIONS := $(PANDOC_OPTIONS) --pdf-engine=$(PDF_ENGINE) \
	--lua-filter=$(MERMAID_FILTER) \
	--lua-filter=$(TABLE_FILTER) \
	--lua-filter=$(SCENARIO_FILTER) \
	--lua-filter=$(TAGLINE_FILTER) \
	--include-in-header=$(STYLE) \
	-V documentclass=report \
	-V classoption=oneside \
	-V papersize=a4 \
	-V geometry:margin=1in \
	-V pagestyle=plain \
	-V header-includes='$(HEADER_INCLUDES)'

.PHONY: all pdf clean distclean help mermaid-cli

## all: Build the PDF
all: pdf

## pdf: Build a single PDF from the chapter markdown files
pdf: $(OUTPUT)

$(OUTPUT): $(SOURCES) $(FILTERS) $(STYLE) $(MERMAID_CONFIG) $(MERMAID_LAYOUT) | $(TARGET_DIR)
	@test -n "$(PDF_ENGINE)" || { echo "No PDF engine found: install MacTeX (xelatex) or tectonic" >&2; exit 1; }
	MMDC="$(MMDC)" MERMAID_DIR="$(BUILD_DIR)" MERMAID_CONFIG="$(MERMAID_CONFIG)" MERMAID_LAYOUT="$(MERMAID_LAYOUT)" \
		pandoc $(PANDOC_PDF_OPTIONS) -o "$@" $(SOURCES)
	@echo "Generated $@"

# The preface is index.md up to its chapter link list, retitled.
$(PREFACE): index.md | $(BUILD_DIR)
	@sed -e '/^## Chapters$$/,$$d' -e '1s|^# .*|# Preface|' index.md > "$@"

$(TARGET_DIR) $(BUILD_DIR):
	@mkdir -p "$@"

## mermaid-cli: Install a local mermaid renderer so the appendix diagrams render
mermaid-cli: | $(BUILD_DIR)
	@mkdir -p "$(BUILD_DIR)"
	npm install --prefix "$(BUILD_DIR)" @mermaid-js/mermaid-cli
	@echo "Installed. `make pdf` now finds it automatically."

## clean: Remove the generated PDF and build intermediates
clean:
	rm -f "$(OUTPUT)"
	rm -f "$(BUILD_DIR)"/*.md "$(BUILD_DIR)"/*.mmd "$(BUILD_DIR)"/*.png
	@echo "Cleaned generated output"

## distclean: Also remove the local mermaid renderer
distclean:
	rm -rf "$(TARGET_DIR)"
	@echo "Removed $(TARGET_DIR)"

## help: Show available make targets
help:
	@echo "The Metasystem PDF build"
	@echo ""
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/^## /  /'
	@echo ""
	@echo "PDF engine: $(if $(PDF_ENGINE),$(PDF_ENGINE),none found)"
	@echo "Mermaid:    $(if $(MMDC),$(MMDC),not found - diagrams stay as source)"
