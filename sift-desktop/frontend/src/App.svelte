<script>
    import { EventsOn } from '../wailsjs/runtime/runtime';
    import { SelectAndIndexFolder, SearchAndAsk } from '../wailsjs/go/main/App';
    import { fade, fly } from 'svelte/transition';

    let query = "";
    let answer = "";
    let path = "";
    let status = "就绪";
    let syncMessage = ""; // 顶部弹窗消息
    let isSearching = false;

    // 1. 监听新文件同步
    EventsOn("file_synced", (msg) => {
        syncMessage = msg;
        setTimeout(() => { syncMessage = ""; }, 3000); // 3秒后消失
    });

    async function handleSearch() {
        if (!query) return;

        answer = "";
        isSearching = true;

        status = "🔍 正在翻阅本地文档...";
        // 模拟一下检索到思考的切换感
        setTimeout(() => { if(isSearching) status = "🧠 Sift 正在思考中..."; }, 800);

        try {
            const res = await SearchAndAsk(query);
            status = "✍️ Sift 回答中...";

            // 模拟打字机效果或直接显示
            answer = res;
            status = "✅ Sift 已回答完";
        } catch (e) {
            status = "❌ 出错了";
            answer = "错误原因: " + e;
        } finally {
            isSearching = false;
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

        <section class="display-container">
            {#if answer}
                <div class="answer-box" in:fade={{ duration: 600 }}>
                    <div class="meta">RESPONSE</div>
                    <p>{answer}</p>
                </div>
            {:else if !isSearching}
                <div class="empty-state">
                    <h2>Sift</h2>
                    <p>你的离线隐私知识库已就绪</p>
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
    /* 极致黑白极简风格 */
    :global(body) { background: #ffffff; color: #121212; font-family: "Inter", system-ui, sans-serif; }

    .sift-app { height: 100vh; display: flex; flex-direction: column; align-items: center; }
    .content-wrapper { width: 100%; max-width: 720px; height: 100%; display: flex; flex-direction: column; padding: 20px; }

    /* 同步提示弹窗 */
    .toast-notification {
        position: fixed; top: 20px; background: #121212; color: #fff;
        padding: 10px 20px; border-radius: 30px; font-size: 13px;
        box-shadow: 0 10px 25px rgba(0,0,0,0.1); z-index: 1000;
    }

    .navbar { display: flex; justify-content: space-between; padding: 20px 0; border-bottom: 1px solid #f0f0f0; }
    .path-badge { cursor: pointer; font-size: 12px; font-weight: 500; display: flex; align-items: center; gap: 8px; }
    .dot { width: 8px; height: 8px; background: #00c853; border-radius: 50%; }
    .status-tag { font-size: 12px; color: #888; transition: color 0.3s; }
    .status-tag.active { color: #121212; font-weight: 600; }

    .display-container { flex: 1; padding: 40px 0; overflow-y: auto; }
    .answer-box { background: #fafafa; padding: 30px; border-radius: 20px; border: 1px solid #eee; }
    .meta { font-size: 10px; font-weight: 800; color: #bbb; letter-spacing: 1px; margin-bottom: 15px; }
    .answer-box p { line-height: 1.8; margin: 0; font-size: 16px; }

    .input-pill {
        background: #fff; border: 1px solid #e0e0e0; border-radius: 16px;
        display: flex; padding: 8px; box-shadow: 0 4px 20px rgba(0,0,0,0.03);
        transition: border-color 0.3s;
    }
    .input-pill:focus-within { border-color: #121212; }
    input { flex: 1; border: none; padding: 12px 20px; outline: none; font-size: 16px; }
    .action-btn {
        background: #121212; color: #fff; border: none; width: 44px; height: 44px;
        border-radius: 12px; cursor: pointer; font-size: 18px;
    }

    .empty-state { text-align: center; margin-top: 20vh; color: #ccc; }
    .empty-state h2 { font-size: 48px; font-weight: 200; margin: 0; color: #121212; }
</style>