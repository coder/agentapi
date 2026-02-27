# AgentAPI Chat Interface

A simple ChatGPT-like interface for AgentAPI. It's a demo showcasing how to use AgentAPI. 95% of the code was generated with Claude Code.

## Development Setup

1. Ensure the AgentAPI backend server is running on `localhost:3284`. You can run it from the root of the repository with e.g.

   ```bash
   go run main.go server -- claude
   ```

2. Install dependencies:

   ```bash
   bun install
   ```

3. Start the development server:

   ```bash
   bun run dev
   ```

4. Open <http://localhost:3000/chat/?url=http://localhost:3284> in your browser
