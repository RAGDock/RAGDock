<script>
    import { EventsOn } from '../wailsjs/runtime/runtime';
    import { SelectAndIndexFolder, SearchAndAsk } from '../wailsjs/go/main/App';
    import { fade, fly } from 'svelte/transition';
    import { marked } from 'marked';
    import { tick } from 'svelte';

    let query = "";
    let path = "";
    let status = "就绪";
    let syncMessage = "";
    let isSearching = false;
    let messages = [];
    let scrollContainer;

    async function scrollToBottom() {
        await tick();
        if (scrollContainer) {
            scrollContainer.scrollTo({
                top: scrollContainer.scrollHeight,
                behavior: 'smooth'
            });
        }
    }

    EventsOn("file_synced", (msg) => {
        syncMessage = msg;
        setTimeout(() => { syncMessage = ""; }, 3000);
    });

    async function handleSearch() {
        if (!query || isSearching) return;
        const userQuery = query;
        query = "";

        messages = [...messages, { role: "user", content: userQuery }];
        await scrollToBottom();

        isSearching = true;
        status = "🧠 Sift 正在思考中...";

        try {
            const res = await SearchAndAsk(userQuery, messages.slice(0, -1));
            messages = [...messages, { role: "assistant", content: res }];
            status = "✅ 已回答";
        } catch (e) {
            status = "❌ 出错了";
            messages = [...messages, { role: "assistant", content: "发生错误: " + e }];
        } finally {
            isSearching = false;
            await scrollToBottom();
        }
    }
</script>

<main class="sift-app">
    {#if syncMessage}
        <div class="toast-notification" in:fly={{ y: -20, duration: 400 }} out:fade>
            {syncMessage}
        </div>
    {/if}

    <div class="content-wrapper">
        <header class="navbar">
            <div class="path-badge" on:click={() => SelectAndIndexFolder().then(p => path = p)}>
                <span class="dot"></span> {path || "关联知识库"}
            </div>
            <div class="status-tag" class:active={isSearching}>{status}</div>
        </header>

        <section class="display-container" bind:this={scrollContainer}>
            {#if messages.length === 0}
                <div class="empty-state">
                    <h2>Sift</h2>
                    <p>你的离线隐私知识库已就绪</p>
                </div>
            {/if}

            {#each messages as msg}
                <div class="message-row {msg.role}" in:fade={{ duration: 300 }}>
                    <div class="meta">{msg.role === 'user' ? 'YOU' : 'SIFT'}</div>
                    <div class="content">
                        {@html marked.parse(msg.content)}
                    </div>
                </div>
            {/each}

            {#if isSearching}
                <div class="thinking-wrapper" in:fade={{ duration: 200 }} out:fade={{ duration: 100 }}>
                    <div class="message-row assistant thinking">
                        <div class="typing-indicator">
                            <span></span><span></span><span></span>
                        </div>
                    </div>
                </div>
            {/if}
        </section>

        <footer class="input-section">
            <div class="input-pill">
                <input
                        bind:value={query}
                        placeholder="向你的本地文档提问..."
                        on:keypress={e => e.key === 'Enter' && handleSearch()}
                />
                <button class="action-btn" on:click={handleSearch} disabled={isSearching}>
                    {isSearching ? "●" : "→"}
                </button>
            </div>
        </footer>
    </div>
</main>

<style>
    :global(html), :global(body) {
        background: #ffffff;
        color: #121212;
        margin: 0;
        padding: 0;
        overflow: hidden;
        width: 100%;
        height: 100%;
    }

    :global(*) { box-sizing: border-box; }

    .sift-app {
        height: 100vh;
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
    }

    .content-wrapper {
        width: 100%;
        max-width: 800px;
        height: 100%;
        display: flex;
        flex-direction: column;
        padding: 0 20px;
    }

    .navbar {
        display: flex;
        justify-content: space-between;
        padding: 20px 0;
        border-bottom: 1px solid #f0f0f0;
        flex-shrink: 0;
    }

    .display-container {
        flex: 1;
        padding: 30px 0;
        overflow-y: auto;
        overflow-x: hidden;
        display: flex;
        flex-direction: column;
        gap: 32px;
        text-align: left;
    }

    .message-row { width: 100%; }

    /* 用户提问保持左侧条状 */
    .message-row.user {
        border-left: 2px solid #121212;
        padding-left: 20px;
        margin-bottom: 8px;
    }

    /* 回答卡片：正常状态下全宽左对齐 */
    .message-row.assistant {
        background: #fafafa;
        padding: 25px;
        border-radius: 20px;
        border: 1px solid #eee;
        text-align: left;
    }

    /* ✅ 思考状态：靠右、窄宽度、圆角矩形 */
    .thinking-wrapper {
        display: flex;
        justify-content: flex-end; /* 靠右对齐 */
        width: 100%;
    }

    .message-row.assistant.thinking {
        width: auto;
        min-width: 80px;
        padding: 15px 25px;
        border-radius: 20px 20px 5px 20px; /* 类似聊天气泡的非对称圆角 */
        background: #f0f0f0;
        border: none;
    }

    .meta { font-size: 10px; font-weight: 800; color: #bbb; letter-spacing: 1px; margin-bottom: 12px; }

    /* 思考动画 */
    .typing-indicator {
        display: flex;
        gap: 5px;
        justify-content: center;
    }

    .typing-indicator span {
        width: 6px;
        height: 6px;
        background: #999;
        border-radius: 50%;
        animation: bounce 1.4s infinite ease-in-out both;
    }

    .typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
    .typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

    @keyframes bounce {
        0%, 80%, 100% { transform: scale(0); }
        40% { transform: scale(1.0); }
    }

    /* Markdown 排版补丁 */
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
</style>