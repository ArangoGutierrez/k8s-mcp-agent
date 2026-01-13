#!/bin/bash
# ralph-loop.sh - Autonomous iteration wrapper for AI agent prompts
# Implements the "Ralph Wiggum" pattern for long-running autonomous tasks
#
# Usage: ./ralph-loop.sh <prompt-file.md> [max-iterations] [agent-command]
#
# Examples:
#   ./ralph-loop.sh docs/prompts/my-task.md
#   ./ralph-loop.sh docs/prompts/my-task.md 10
#   ./ralph-loop.sh docs/prompts/my-task.md 10 "claude --print"
#
# The script will:
# 1. Run the agent with the prompt file
# 2. Check for completion signal in output
# 3. Check for completion signal in the prompt file itself
# 4. Re-invoke if tasks remain incomplete
# 5. Stop after max iterations or completion

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROMPT_FILE="${1:-}"
MAX_ITERATIONS="${2:-10}"
AGENT_CMD="${3:-}"
COMPLETION_MARKER="ALL_TASKS_DONE"
SLEEP_BETWEEN_ITERATIONS=2

# Usage help
usage() {
    echo "Usage: $0 <prompt-file.md> [max-iterations] [agent-command]"
    echo ""
    echo "Arguments:"
    echo "  prompt-file.md    Path to the prompt file (required)"
    echo "  max-iterations    Maximum iterations to run (default: 10)"
    echo "  agent-command     Command to run the agent (auto-detected if not provided)"
    echo ""
    echo "Examples:"
    echo "  $0 docs/prompts/my-task.md"
    echo "  $0 docs/prompts/my-task.md 15"
    echo "  $0 docs/prompts/my-task.md 10 'claude --print'"
    echo ""
    echo "The script looks for '<completion>ALL_TASKS_DONE</completion>' in the"
    echo "agent output or 'completion_signal: \"ALL_TASKS_DONE\"' in the prompt file."
    exit 1
}

# Validate arguments
if [[ -z "$PROMPT_FILE" ]]; then
    echo -e "${RED}Error: Prompt file is required${NC}"
    usage
fi

if [[ ! -f "$PROMPT_FILE" ]]; then
    echo -e "${RED}Error: Prompt file not found: $PROMPT_FILE${NC}"
    exit 1
fi

# Auto-detect agent command if not provided
detect_agent() {
    if [[ -n "$AGENT_CMD" ]]; then
        echo "$AGENT_CMD"
        return
    fi
    
    # Check for common AI agent CLIs
    if command -v claude &> /dev/null; then
        echo "claude --print"
    elif command -v cursor-agent &> /dev/null; then
        echo "cursor-agent --file"
    elif command -v aider &> /dev/null; then
        echo "aider --message-file"
    else
        echo ""
    fi
}

AGENT_CMD=$(detect_agent)

if [[ -z "$AGENT_CMD" ]]; then
    echo -e "${YELLOW}Warning: No agent CLI detected.${NC}"
    echo "Please provide the agent command as the third argument."
    echo ""
    echo "Examples:"
    echo "  $0 $PROMPT_FILE $MAX_ITERATIONS 'claude --print'"
    echo "  $0 $PROMPT_FILE $MAX_ITERATIONS 'your-agent-cli'"
    exit 1
fi

# Print banner
print_banner() {
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════════════╗"
    echo "║           🔁 RALPH WIGGUM AUTONOMOUS LOOP                            ║"
    echo "╠══════════════════════════════════════════════════════════════════════╣"
    echo "║  Prompt:     $PROMPT_FILE"
    echo "║  Max Iters:  $MAX_ITERATIONS"
    echo "║  Agent:      $AGENT_CMD"
    echo "║  Completion: <completion>$COMPLETION_MARKER</completion>"
    echo "╚══════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Check if completion signal is in output
check_output_completion() {
    local output="$1"
    if echo "$output" | grep -q "<completion>$COMPLETION_MARKER</completion>"; then
        return 0
    fi
    return 1
}

# Check if completion signal is in prompt file
check_file_completion() {
    if grep -q "completion_signal: \"$COMPLETION_MARKER\"" "$PROMPT_FILE" 2>/dev/null; then
        return 0
    fi
    return 1
}

# Count tasks by status
count_tasks() {
    local status="$1"
    grep -c "status: \"\[$status\]\"" "$PROMPT_FILE" 2>/dev/null || echo "0"
}

# Print task summary
print_task_summary() {
    local todo=$(count_tasks "TODO")
    local wip=$(count_tasks "WIP")
    local done=$(count_tasks "DONE")
    local blocked=$(grep -c "status: \"\[BLOCKED:" "$PROMPT_FILE" 2>/dev/null || echo "0")
    
    echo -e "${BLUE}Task Summary:${NC}"
    echo -e "  TODO:    $todo"
    echo -e "  WIP:     $wip"
    echo -e "  DONE:    $done"
    echo -e "  BLOCKED: $blocked"
}

# Update iteration counter in prompt file
update_iteration() {
    local iter="$1"
    if grep -q "^iteration:" "$PROMPT_FILE"; then
        # macOS and GNU sed compatible
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s/^iteration:.*/iteration: $iter/" "$PROMPT_FILE"
        else
            sed -i "s/^iteration:.*/iteration: $iter/" "$PROMPT_FILE"
        fi
    fi
}

# Main loop
main() {
    print_banner
    
    local start_time=$(date +%s)
    
    for i in $(seq 1 "$MAX_ITERATIONS"); do
        echo ""
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${CYAN}🔁 ITERATION $i of $MAX_ITERATIONS${NC}"
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo ""
        
        # Update iteration counter in file
        update_iteration "$i"
        
        # Print current task status
        print_task_summary
        echo ""
        
        # Run the agent
        echo -e "${BLUE}Running agent: $AGENT_CMD $PROMPT_FILE${NC}"
        echo ""
        
        local output
        if ! output=$($AGENT_CMD "$PROMPT_FILE" 2>&1); then
            echo -e "${YELLOW}Warning: Agent exited with non-zero status${NC}"
        fi
        
        echo "$output"
        
        # Check for completion in output
        if check_output_completion "$output"; then
            echo ""
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo -e "${GREEN}✅ COMPLETION DETECTED IN OUTPUT at iteration $i${NC}"
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            echo -e "Total time: ${duration}s"
            print_task_summary
            exit 0
        fi
        
        # Check for completion in file
        if check_file_completion; then
            echo ""
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo -e "${GREEN}✅ COMPLETION DETECTED IN FILE at iteration $i${NC}"
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            echo -e "Total time: ${duration}s"
            print_task_summary
            exit 0
        fi
        
        # Check if all tasks are done (fallback)
        local todo=$(count_tasks "TODO")
        local wip=$(count_tasks "WIP")
        if [[ "$todo" == "0" && "$wip" == "0" ]]; then
            echo ""
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo -e "${GREEN}✅ ALL TASKS DONE at iteration $i${NC}"
            echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            echo -e "Total time: ${duration}s"
            print_task_summary
            exit 0
        fi
        
        echo ""
        echo -e "${YELLOW}⏳ Tasks still pending, continuing to next iteration...${NC}"
        print_task_summary
        
        if [[ $i -lt $MAX_ITERATIONS ]]; then
            echo -e "Sleeping ${SLEEP_BETWEEN_ITERATIONS}s before next iteration..."
            sleep "$SLEEP_BETWEEN_ITERATIONS"
        fi
    done
    
    echo ""
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}⚠️  MAX ITERATIONS ($MAX_ITERATIONS) reached without completion${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    print_task_summary
    echo ""
    echo "Review the prompt file for remaining tasks:"
    echo "  grep -E '\\[TODO\\]|\\[WIP\\]|\\[BLOCKED:' $PROMPT_FILE"
    exit 1
}

main "$@"
