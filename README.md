# smartcommit

**smartcommit** is an intelligent, AI-powered CLI tool that helps you write semantic, Conventional Commits messages effortlessly. It analyzes your staged changes, asks clarifying questions to understand the "why" behind your code, and generates a structured commit message for you.

Future you will thank you for deciding to use `smartcommit`!

![smartcommit Demo](./demo.gif)

## Features

- **AI-Powered Analysis**: Automatically analyzes your staged `git diff` to understand what changed.
- **Interactive Q&A**: Asks you specific, relevant questions to gather context that isn't obvious from the code alone (the "why" and "intent").
- **Multi-Provider Support**:
    -   **OpenAI (GPT-4o)**: For top-tier accuracy and performance.
    -   **Ollama (Llama 3.1)**: Run locally and privately for free.
- **Conventional Commits**: strictly enforces the [Conventional Commits](https://www.conventionalcommits.org/) specification (`feat`, `fix`, `chore`, etc.).
- **Beautiful TUI**: A responsive, easy-to-use Terminal User Interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Installation

### Prerequisites

-   **Go** (1.21 or later)
-   **Git** installed and available in your PATH.
-   *(Optional)* **Ollama** installed locally if you plan to use the local model.

```bash
go install github.com/arpxspace/smartcommit@latest
```

### Build from Source

1.  Clone the repository:
    ```bash
    git clone https://github.com/arpxspace/smartcommit.git
    cd smartcommit
    ```

2.  Build the binary:
    ```bash
    go build -o smartcommit
    ```

3.  Move to your PATH (optional):
    ```bash
    mv smartcommit /usr/local/bin/
    ```

### Use instead of `git commit`

To use `smartcommit` as your default `git commit` command, run:

```bash
git config --global alias.ci '!smartcommit'
```

Or for this repository only:

```bash
git config alias.ci '!smartcommit'
```

**Usage:**
- To commit with smartcommit: `git ci`

## 🚀 Usage

1.  **Stage your changes**:
    ```bash
    git add .
    ```

2.  **Run smartcommit**:
    ```bash
    smartcommit
    ```

3.  **Follow the TUI**:
    -   **First Run**: You'll be asked to choose your AI provider (OpenAI or Ollama) and configure it.
    -   **Analysis**: The AI will analyze your changes.
    -   **Questions**: Answer a few questions to provide context.
    -   **Review**: The AI generates a commit message. You can edit it or confirm it to commit immediately.

### Manual Mode
If you already know what you want to write, you can select **"I already know what to write"** from the main menu to open your default git editor.

## ⚙️ Configuration

smartcommit stores its configuration in a local file (usually `~/.smartcommit/config.json`).

### Environment Variables

-   `OPENAI_API_KEY`: If set, smartcommit can detect this during setup and ask if you want to use it, saving you from pasting it manually.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1.  Fork the project
2.  Create your feature branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your changes (`git commit -m 'feat: Add some AmazingFeature'`)
4.  Push to the branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
