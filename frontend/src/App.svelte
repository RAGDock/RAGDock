<script>
    import { EventsOn } from '../wailsjs/runtime/runtime';
    import { SelectAndIndexFolder, SearchAndAsk, StopSearch } from '../wailsjs/go/main/App';
    import { fade, fly } from 'svelte/transition';
    import { marked } from 'marked';
    import { tick } from 'svelte';

    // Application state management
    let query = "";
    let path = "";
    let status = "Ready";
    let syncMessage = "";
    let isSearching = false; // Tracks if the model is currently searching or generating

    // Conversation structure: { role: 'user'|'assistant', content: '...', thinking: '...' }
    let messages = [];
    let scrollContainer;
    let copiedIndex = null;

    // 1. Listen for streaming LLM tokens
    EventsOn("llm_token", (token) => {
        let lastMsg = messages[messages.length - 1];

        if (lastMsg && lastMsg.role === 'assistant') {
            // Distinguish between reasoning (thinking) and final answer (response)
            if (token.thinking) {
                lastMsg.thinking = (lastMsg.thinking || "") + token.thinking;
            } else if (token.response) {
                lastMsg.content = (lastMsg.content || "") + token.response;
            }
            // Trigger Svelte reactive update
            messages = [...messages];
            scrollToBottom();
        }
    });

    // 2. Listen for background file synchronization events
    EventsOn("file_synced", (msg) => {
        syncMessage = msg;
        setTimeout(() => { syncMessage = ""; }, 3000);
    });

    // Automatically scroll to the bottom of the chat
    async function scrollToBottom() {
        await tick();
        if (scrollContainer) {
            scrollContainer.scrollTo({
                top: scrollContainer.scrollHeight,
                behavior: 'smooth'
            });
        }
    }

    // Handle search query submission
    async function handleSearch() {
        if (!query || isSearching) return;
        const userQuery = query;
        query = "";

        // Add user question to the message list
        messages = [...messages, { role: "user", content: userQuery }];

        // Placeholder for the assistant's streaming response
        messages = [...messages, {
            role: "assistant",
            content: "",
            thinking: ""
        }];

        await scrollToBottom();

        isSearching = true;
        status = "RAGDock is thinking...";

        try {
            // Call the backend with the current query and history (excluding latest turn)
            await SearchAndAsk(userQuery, messages.slice(0, -2));
            status = "Ready";
        } catch (e) {
            // Handle cancellation or errors gracefully
            if (e.includes("canceled")) {
                status = "Halted";
            } else {
                status = "Error";
                let lastMsg = messages[messages.length - 1];
                lastMsg.content = "Error: " + e;
            }
        } finally {
            isSearching = false;
            await scrollToBottom();
        }
    }

    // Manually stop the current search/generation
    async function handleStop() {
        await StopSearch();
        isSearching = false;
        status = "Stopped";
    }

    // Copy message content to clipboard
    async function copyToClipboard(text, index) {
        try {
            await navigator.clipboard.writeText(text);
            copiedIndex = index;
            setTimeout(() => { if (copiedIndex === index) copiedIndex = null; }, 2000);
        } catch (err) {
            console.error('Failed to copy: ', err);
        }
    }
</script>

<main class="RAGDock-app">
    {#if syncMessage}
        <div class="toast-notification" in:fly={{ y: -20, duration: 400 }} out:fade>
            {syncMessage}
        </div>
    {/if}

    <div class="content-wrapper">
        <header class="navbar">
            <div class="path-badge" on:click={() => SelectAndIndexFolder().then(p => path = p)}>
                <span class="dot"></span> {path || "Connect Knowledge Base"}
            </div>
            <div class="status-tag" class:active={isSearching}>{status}</div>
        </header>

        <section class="display-container" bind:this={scrollContainer}>
            {#if messages.length === 0}
                <div class="empty-state">
                    <h2>RAGDock</h2>
                    <p>Your private offline knowledge base is ready.</p>
                </div>
            {/if}

            {#each messages as msg, i}
                <div class="message-row {msg.role}" in:fade={{ duration: 300 }}>
                    <div class="message-header">
                        <div class="meta">{msg.role === 'user' ? 'YOU' : 'RAGDock'}</div>
                        {#if msg.role === 'assistant' && msg.content}
                            <button class="copy-btn" on:click={() => copyToClipboard(msg.content, i)}>
                                {copiedIndex === i ? 'COPIED!' : 'COPY'}
                            </button>
                        {/if}
                    </div>

                    <div class="content">
                        {#if msg.thinking}
                            <details class="thinking-box" open={isSearching && i === messages.length - 1}>
                                <summary>Thinking Chain</summary>
                                <div class="thinking-text">{msg.thinking}</div>
                            </details>
                        {/if}

                        {#if msg.content}
                            <div class="markdown-body">
                                {@html marked.parse(msg.content)}
                            </div>
                        {/if}
                    </div>
                </div>
            {/each}

            {#if isSearching && (!messages[messages.length - 1]?.thinking && !messages[messages.length - 1]?.content)}
                <div class="thinking-wrapper" in:fade={{ duration: 200 }} out:fade={{ duration: 100 }}>
                    <div class="message-row assistant thinking">
                        <div class="meta">RAGDock</div>
                        <div class="content">
                            <div class="typing-indicator">
                                <span></span><span></span><span></span>
                            </div>
                        </div>
                    </div>
                </div>
            {/if}
        </section>

        <footer class="input-section">
            <div class="input-pill" class:searching={isSearching}>
                <input
                        bind:value={query}
                        placeholder={isSearching ? "Generating response..." : "Ask your documents..."}
                        on:keypress={e => e.key === 'Enter' && !isSearching && handleSearch()}
                        disabled={isSearching}
                />

                {#if isSearching}
                    <button class="stop-btn" on:click={handleStop} in:fade>
                        <div class="stop-icon"></div>
                    </button>
                {:else}
                    <button class="action-btn" on:click={handleSearch} disabled={!query} in:fade>
                        <span>→</span>
                    </button>
                {/if}
            </div>
        </footer>
    </div>
</main>

<style>
    /* CSS Styles - Maintained for layout consistency */
    :global(html), :global(body) {
        background: #ffffff; color: #121212; margin: 0; padding: 0;
        overflow: hidden; width: 100%; height: 100%;
    }
    :global(*) { box-sizing: border-box; }

    .RAGDock-app { height: 100vh; width: 100%; display: flex; flex-direction: column; align-items: center; }
    .content-wrapper { width: 100%; max-width: 800px; height: 100%; display: flex; flex-direction: column; padding: 0 20px; }

    .navbar { display: flex; justify-content: space-between; padding: 20px 0; border-bottom: 1px solid #f0f0f0; flex-shrink: 0; }
    .path-badge { cursor: pointer; font-size: 12px; font-weight: 500; display: flex; align-items: center; gap: 8px; }
    .dot { width: 8px; height: 8px; background: #00c853; border-radius: 50%; }
    .status-tag { font-size: 12px; color: #888; }

    .display-container { flex: 1; padding: 30px 0; overflow-y: auto; overflow-x: hidden; display: flex; flex-direction: column; gap: 32px; }

    .message-row { width: 100%; }
    .message-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }

    .message-row.user { border-left: 2px solid #121212; padding-left: 20px; }
    .message-row.assistant { background: #fafafa; padding: 25px; border-radius: 20px; border: 1px solid #eee; text-align: left; }

    .thinking-wrapper { display: flex; justify-content: flex-end; width: 100%; margin-top: -10px; }
    .message-row.assistant.thinking { width: auto; min-width: 100px; padding: 15px 25px; border-radius: 20px 20px 5px 20px; background: #f0f0f0; border: none; }

    .meta { font-size: 10px; font-weight: 800; color: #bbb; letter-spacing: 1px; margin: 0; }

    .typing-indicator { display: flex; gap: 5px; padding-top: 5px; }
    .typing-indicator span {
        width: 6px; height: 6px; background: #999; border-radius: 50%;
        animation: bounce 1.4s infinite ease-in-out both;
    }
    .typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
    .typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

    @keyframes bounce {
        0%, 80%, 100% { transform: scale(0); }
        40% { transform: scale(1.0); }
    }

    .stop-btn {
        background: #000000;
        color: #fff;
        border: none;
        width: 44px;
        height: 44px;
        border-radius: 12px;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: transform 0.1s;
    }
    .stop-btn:active { transform: scale(0.9); }
    .stop-icon {
        width: 14px;
        height: 14px;
        background: white;
        border-radius: 2px;
    }

    .thinking-box {
        margin-bottom: 15px;
        background: #f8f8f8;
        border-radius: 8px;
        border-left: 3px solid #ddd;
        font-size: 13px;
        color: #777;
    }
    .thinking-box summary {
        padding: 8px 12px;
        cursor: pointer;
        font-weight: 600;
        list-style: none;
        user-select: none;
    }
    .thinking-box summary::-webkit-details-marker { display: none; }
    .thinking-text {
        padding: 0 12px 12px;
        line-height: 1.5;
        font-style: italic;
        white-space: pre-wrap;
    }

    .copy-btn {
        background: transparent; border: 1px solid #ddd; color: #999;
        font-size: 10px; font-weight: 700; padding: 4px 8px; border-radius: 6px;
        cursor: pointer; transition: all 0.2s;
    }
    .copy-btn:hover { background: #121212; color: #fff; border-color: #121212; }

    .content :global(p) { line-height: 1.8; font-size: 16px; margin-bottom: 16px; text-align: left; }
    .content :global(ul), .content :global(ol) { padding-left: 1.5em; margin-bottom: 16px; text-align: left; }

    .input-section { padding: 20px 0 30px; flex-shrink: 0; }
    .input-pill {
        background: #fff; border: 1px solid #e0e0e0; border-radius: 16px;
        display: flex; padding: 6px; box-shadow: 0 4px 20px rgba(0,0,0,0.03);
    }
    input { flex: 1; border: none; padding: 12px 15px; outline: none; font-size: 16px; }
    .action-btn { background: #121212; color: #fff; border: none; width: 44px; height: 44px; border-radius: 12px; cursor: pointer; }

    .empty-state { text-align: center; margin-top: 10vh; }
    .empty-state h2 { font-size: 56px; font-weight: 200; margin: 0; color: #121212; }

    .toast-notification {
        position: fixed; top: 20px; left: 50%; transform: translateX(-50%);
        background: #121212; color: #fff; padding: 10px 20px; border-radius: 10px;
        font-size: 12px; font-weight: 600; z-index: 1000; box-shadow: 0 10px 30px rgba(0,0,0,0.1);
    }
</style>
