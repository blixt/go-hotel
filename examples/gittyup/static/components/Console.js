import htm from "htm";
import React from "react";

const html = htm.bind(React.createElement);

export function Console({ logs, chatInput, setChatInput, handleChatSubmit, connectionState, logsRef }) {
    return html`
        <div className="border-t border-slate-300 dark:border-slate-700">
            <div className="flex flex-col h-64">
                <div
                    ref=${logsRef}
                    className="flex-1 overflow-y-auto p-3 font-mono text-xs bg-slate-100 dark:bg-slate-900 text-slate-900 dark:text-slate-200"
                >
                    ${logs.map((log, index) => html`<div key=${index} className="whitespace-pre-wrap break-words">${log}</div>`)}
                </div>
                <form
                    onSubmit=${handleChatSubmit}
                    className="flex border-t border-slate-300 dark:border-slate-700"
                >
                    <input
                        type="text"
                        value=${chatInput}
                        onChange=${(e) => setChatInput(e.target.value)}
                        placeholder="Type a message..."
                        disabled=${connectionState === "disconnected"}
                        className="flex-1 px-3 py-2 bg-white dark:bg-slate-800 text-sm focus:outline-none text-slate-900 dark:text-slate-200 disabled:bg-slate-100 dark:disabled:bg-slate-900"
                    />
                </form>
            </div>
        </div>
    `;
}
