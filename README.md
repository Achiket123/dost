# DOST - Developer Orchestrator System Tool

## Overview
DOST is an autonomous AI agent designed to streamline software development workflows. It acts as a central intelligence and coordination hub, routing tasks to specialized agents and executing commands to achieve user objectives efficiently.

## Core Capabilities

*   **Direct Command Execution:** Executes standard CLI commands immediately for file operations, process management, network utilities, and system queries.
*   **Intelligent Task Routing:**
    *   **Analysis Agent:** Routes tasks requiring error log interpretation, project structure assessment, dependency conflict resolution, performance issue analysis, or security vulnerability identification.
    *   **Coder Agent:** Routes tasks involving specific code requirements, file modifications, bug fixes, feature implementation, or code refactoring.
    *   **Planner Agent:** Creates step-by-step execution plans for complex, multi-stage tasks.
*   **Autonomous Operation:** Infers information from context, applies standard solutions, makes reasonable assumptions, and clarifies ambiguous requests with minimal user interruption.
*   **Context Awareness:** Adapts behavior based on project type, development stage, user skill level, system environment, and tool availability.

## Key Features

*   **Parallel Processing:** Executes tasks concurrently (e.g., testing during building, documentation generation during compilation).
*   **Intelligent Caching:** Remembers successful command sequences, caches analysis results, and reuses planning outputs.
*   **Failure Recovery:** Retries failed commands, selects alternative approaches, and provides clear failure explanations with recovery options.
*   **Performance Monitoring:** Tracks task completion times, monitors agent efficiency, and optimizes routing decisions.

## Usage

DOST is designed to be a versatile tool for a wide range of development tasks. Simply provide a clear objective, and DOST will handle the rest, from analyzing the requirements to executing the necessary steps.

## Examples

*   `update the readme.md stating your capabilities and push it to github`
*   `install the required dependencies`
*   `run the tests`

## Communication

DOST provides progress updates, error reports with root cause analysis, success confirmations, and resource usage awareness.

## Contributing

[Contribution guidelines will be added here]

## License

[License information will be added here]
