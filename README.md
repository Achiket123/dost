# DOST - The Elite AI Coding Assistant

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Overview

DOST is an autonomous AI coding assistant meticulously engineered to surpass the capabilities of tools like Cursor, Claude Code, and Firebase Studio. It's designed for complete self-sufficiency in complex software development tasks, spanning from initial project inception to seamless production deployment. Embodying a 'Master Craftsman' mindset, DOST is driven by a relentless pursuit of first-attempt success, zero-friction development workflows, and the generation of production-ready code.

## :rocket: Core Capabilities

*   **Autonomous Project Initialization:** Define your project goals, and DOST will autonomously set up the project structure, configure build systems, and establish essential development environments.
*   **Intelligent Code Generation:** DOST crafts idiomatic, robust code tailored to your project's language, framework, and architectural patterns. This includes automatic implementation of error handling, comprehensive logging, and rigorous testing.
*   **Automated Dependency Management:** DOST intelligently resolves dependencies, manages conflicts, and ensures seamless integration of required libraries and packages.
*   **Real-Time Error Diagnosis & Resolution:** Leveraging a comprehensive error pattern database, DOST proactively identifies, diagnoses, and resolves errors, often before they manifest as runtime issues.
*   **Performance Optimization:** DOST analyzes and optimizes code for peak performance, employing strategies like caching, compression, lazy loading, and efficient data structures.
*   **Security-First Development:** DOST automatically implements security best practices, encompassing authentication, authorization, data protection, and robust infrastructure security measures.
*   **Comprehensive Testing Automation:** DOST generates a suite of tests, including unit, integration, end-to-end, performance, and security tests, ensuring code reliability and resilience.
*   **Streamlined Deployment & DevOps:** DOST auto-generates deployment configurations for platforms like Docker and Kubernetes, automating CI/CD pipelines for seamless releases.
*   **Continuous Learning & Adaptation:** DOST continuously learns from user interactions, emerging technologies, and codebase analysis, adapting its strategies for optimal performance and effectiveness.

## Agent Capabilities

*   **Analysis Agent:** Performs in-depth task analysis and validation. It identifies potential issues, clarifies requirements, and ensures the feasibility of proposed solutions. It excels at:
    *   Interpreting error logs and stack traces.
    *   Assessing project structure and identifying potential areas of improvement.
    *   Detecting dependency conflicts and suggesting resolutions.
    *   Identifying performance bottlenecks and recommending optimization strategies.
    *   Recognizing security vulnerabilities and proposing remediation measures.
*   **Coder Agent:** Implements code based on specifications. It generates clean, efficient, and well-documented code, adhering to best practices and project standards. It is skilled at:
    *   Writing new code from scratch based on detailed requirements.
    *   Modifying existing code to fix bugs or add new features.
    *   Refactoring code to improve readability, maintainability, and performance.
    *   Generating unit tests and integration tests to ensure code quality.
    *   Creating documentation to explain code functionality and usage.

## :brain: Intelligent Inference Engine

*   **Project Context Detection:** Automatically detects project context from minimal signals (e.g., file types, package dependencies).
*   **Smart Defaults Matrix:** Applies intelligent defaults for versions, architecture (SOLID principles, clean architecture), security, performance, testing, and DevOps.

## :gear: Autonomous Task Execution

*   **Level 1: Instant Commands (0-5 seconds):** Executes immediately without analysis (e.g., Git operations, package management, build commands, file operations, system queries).
*   **Level 2: Smart Execution (5-30 seconds):** Auto-configures and executes with intelligent defaults (e.g., project initialization, dependency resolution, build system configuration, environment setup, database schema generation).
*   **Level 3: Complex Problem Solving (30 seconds - 5 minutes):** Full analysis, planning, and implementation (e.g., multi-service architecture design, performance optimization, integration testing, security audits).

## :file_folder: Configuration

DOST uses a configuration file to store settings. The file is located at `.dost.yaml` in the root directory or `configs/.dost.yaml` for development environment. Here are the configurable fields:

### App Configuration

*   `app.name`: The name of the application (default: dost).
*   `app.version`: The version of the application (default: 1.0.0).
*   `app.port`: The port the application listens on (default: 8080).
*   `app.debug`: Enable debug mode (default: false).

### AI Configuration

*   `ai.API_KEY`: The API key for the Gemini API. **Required**.
*   `ai.ORG`: The organization for the Gemini API.
*   `ai.MODEL`: The model to use for the Gemini API.
*   `ai.CODER_URL`: URL for the coder service
*   `ai.CODER_API`: API key for the coder service
*   `ai.CODER_ORG`: Organization for the coder service
*   `ai.PLANNER_URL`: URL for the planner service
*   `ai.PLANNER_API`: API key for the planner service
*   `ai.PLANNER_ORG`: Organization for the planner service
*   `ai.ORCHESTRATOR_URL`: URL for the orchestrator service
*   `ai.ORCHESTRATOR_API`: API key for the orchestrator service
*   `ai.ORCHESTRATOR_ORG`: Organization for the orchestrator service
*   `ai.CRITIC_URL`: URL for the critic service
*   `ai.CRITIC_API`: API key for the critic service
*   `ai.CRITIC_ORG`: Organization for the critic service
*   `ai.EXECUTOR_URL`: URL for the executor service
*   `ai.EXECUTOR_API`: API key for the executor service
*   `ai.EXECUTOR_ORG`: Organization for the executor service
*   `ai.KNOWLEDGE_URL`: URL for the knowledge service
*   `ai.KNOWLEDGE_API`: API key for the knowledge service
*   `ai.KNOWLEDGE_ORG`: Organization for the knowledge service
*   `ai.TEMPERATURE`: The temperature for the Gemini API.
*   `ai.MAX_TOKENS`: The maximum number of tokens for the Gemini API.
*   `ai.TOP_P`: The top_p for the Gemini API.

### Logger Configuration

*   `logger.level`: The log level (default: info).
*   `logger.format`: The log format (default: json).

## :file_folder: Universal Project Templates

DOST utilizes universal project templates for consistency and rapid development. Examples include:

*   **Full-Stack Application Template** (Frontend, Backend, Infrastructure, Documentation, CI/CD)
*   **Microservices Architecture Template** (API Gateway, User Service, Data Service, Notification Service, Shared Components)

## :hammer: Getting Started

To get started with DOST, follow these steps:

1.  **Prerequisites:**
    *   Go (version 1.21 or higher)
    *   Git
    *   Docker (optional, for containerization)
2.  **Installation:**
    ```bash
    go install github.com/your-username/dost@latest
    ```
    *Replace `github.com/your-username/dost` with the actual repository path.*
3.  **Configuration:**
    *   Create a `.dost.yaml` file in the root directory or `configs/.dost.yaml` for development environment. See example configurations below.

Example `.dost.yaml` for production:

```yaml
# .dost.yaml
api_key: "YOUR_API_KEY" #Required
model: "gpt-4"          #Optional

#Optional settings
settings:
    temp_dir: "/tmp/dost"  #Temporary directory
    log_level: "info"
```

Example `configs/.dost.yaml` for development:

```yaml
# configs/.dost.yaml
app:
  name: "dost"
  version: "1.0.0"
  port: 8080
  debug: true
ai:
  API_KEY: "YOUR_API_KEY"
  ORG: "YOUR_ORG"
  MODEL: "YOUR_MODEL"
logger:
  level: "info"
  format: "json"
```

## :handshake: Contributing

Contributions are welcome! Here are several ways you can contribute:

*   **Report Bugs:** Submit bug reports through GitHub Issues.
*   **Suggest Enhancements:** Share your ideas for new features or improvements.
*   **Write Code:** Contribute code fixes or new features by submitting a pull request.

When contributing code, please follow these guidelines:

*   Ensure your code adheres to the project's coding standards.
*   Write clear, concise commit messages.
*   Include relevant tests for your changes.
*   Document any new or modified functionality.

## :copyright: License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
