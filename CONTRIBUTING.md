# Contributing to Boardgame Night Bot

Thank you for your interest in contributing to the Boardgame Night Bot project! We welcome contributions in the form of code, documentation, bug reports, and feature requests. Please follow the guidelines below to ensure a smooth collaboration.

## Getting Started

1. **Fork the Repository**: Click the "Fork" button on the repository to create your own copy.
2. **Clone the Repository**: Clone your forked repository to your local machine:
   ```sh
   git clone https://github.com/your-username/boardgame-night-bot.git
   cd boardgame-night-bot
   ```
3. **Create a Branch**: Create a new branch for your feature or bug fix:
   ```sh
   git checkout -b feature/name
   ```
4. **Install Dependencies**: Ensure you have Go installed and run:
   ```sh
   go mod tidy
   ```
5. **Set Up Your Environment**:
   - Ensure you have a valid database connection.
   - Set up the required API keys and environment variables as described in the README.md.

## Code Contributions

### Code Style
- Follow Go best practices.
- Use meaningful variable and function names.
- Maintain a consistent coding style.

### Adding Features or Fixing Bugs
1. **Check Open Issues**: Look through existing issues to see if your contribution aligns with ongoing discussions.
2. **Write Clean and Modular Code**: Keep functions and structures well-defined.
3. **Test Your Changes**:
   - Run unit tests:
     ```sh
     go test ./...
     ```
   - Manually test Telegram bot interactions.
4. **Commit Your Changes**:
   - Follow the commit message convention: `type: short description`
   - Example: `fix: correct event creation error`
   ```sh
   git commit -m "feat: add new event creation command"
   ```
5. **Push Your Changes**:
   ```sh
   git push origin feature-name
   ```
6. **Submit a Pull Request**:
   - Open a PR against the `main` branch.
   - Provide a clear description of the changes made.
   - Mention any related issues.

## Reporting Bugs
1. **Search for Existing Issues**: Avoid duplicates by checking if the issue is already reported.
2. **Create a New Issue**:
   - Describe the problem and steps to reproduce.
   - Include expected and actual behavior.
   - Provide relevant logs or screenshots.

## Feature Requests
1. **Describe the Feature**: Explain the motivation and expected outcome.
2. **Provide Examples**: Share how the feature would be used.

## Code Review Process
- PRs will be reviewed by maintainers before merging.
- Address requested changes promptly.
- Squash commits if necessary before merging.

## License
By contributing, you agree that your contributions will be licensed under the project's [MIT License](LICENSE.md).

Thank you for contributing to Boardgame Night Bot!

