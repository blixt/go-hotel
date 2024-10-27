import htm from "htm";
import React, { useReducer, useEffect, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import { ConnectionForm } from "./components/ConnectionForm.js";
import { Console } from "./components/Console.js";
import { Loading } from "./components/Loading.js";
import { useFileContent } from "./hooks.js";
import { parseWebSocketMessage } from "./messages.js";
import { CodeEditor, useSetupMonaco } from "./monaco.js";
import { initialState, reducer } from "./reducer.js";

const html = htm.bind(React.createElement);

function App() {
    useSetupMonaco();
    const [state, dispatch] = useReducer(reducer, initialState);
    const logsRef = useRef(null);
    const [chatInput, setChatInput] = useState("");

    const currentFile = useFileContent(state.repoHash, state.currentCommit, state.selectedFile);

    const handleFileSelect = (path) => {
        dispatch({ type: "SELECT_FILE", path });
    };

    useEffect(() => {
        if (!state.socket) return;

        state.socket.onmessage = (event) => {
            const envelope = parseWebSocketMessage(event.data);

            switch (envelope.type) {
                case "welcome": {
                    dispatch({
                        type: "INITIALIZE",
                        currentUserId: envelope.id,
                        users: envelope.message.users,
                        files: envelope.message.files,
                        repoHash: envelope.message.repoHash,
                        commit: envelope.message.currentCommit,
                    });
                    break;
                }
                case "join":
                    dispatch({ type: "USER_JOINED", user: envelope.message.user });
                    break;
                case "leave":
                    dispatch({ type: "USER_LEFT", id: envelope.id });
                    break;
                case "chat": {
                    dispatch({
                        type: "CHAT_MESSAGE",
                        userId: envelope.id,
                        content: envelope.message.content,
                    });
                    break;
                }
                default:
                    dispatch({ type: "LOG", message: event.data });
            }
        };

        state.socket.onclose = () => {
            dispatch({ type: "DISCONNECTED", error: null });
            dispatch({ type: "LOG", message: "WebSocket connection closed." });
        };

        state.socket.onerror = (error) => {
            dispatch({ type: "DISCONNECTED", error: error.message });
            dispatch({ type: "LOG", message: `WebSocket error: ${error.message}` });
        };
    }, [state.socket]);

    const handleChatSubmit = (e) => {
        e.preventDefault();
        const content = chatInput.trim();
        if (!content || !state.socket) return;
        const message = { content };
        state.socket.send(`chat ${JSON.stringify(message)}`);
        dispatch({
            type: "CHAT_MESSAGE",
            userId: state.user.id,
            content,
        });
        setChatInput("");
    };

    return html`
        <div className="flex flex-col h-screen bg-slate-100 dark:bg-slate-900">
            <div className="flex flex-1 min-h-0">
                <div className="w-80 flex flex-col border-r border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800">
                    <${ConnectionForm} state=${state} dispatch=${dispatch} />

                    <div className="flex-1 overflow-y-auto">
                        <div className="p-4">
                            <h2 className="text-sm font-semibold mb-2 text-slate-700 dark:text-slate-300">Files</h2>
                            <div className="space-y-1">
                                ${state.files.map(
                                    (file) => html`
                                        <button
                                            key=${file}
                                            onClick=${() => handleFileSelect(file)}
                                            className=${`w-full text-left px-2 py-1 text-sm rounded ${
                                                state.selectedFile === file
                                                    ? "bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200"
                                                    : "hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300"
                                            }`}
                                        >
                                            ${file}
                                        </button>
                                    `,
                                )}
                            </div>
                        </div>
                    </div>
                </div>

                <div className="flex-1 min-h-0">
                    ${
                        currentFile.isLoading
                            ? html`<div className="flex items-center justify-center h-full text-slate-800 dark:text-slate-200">
                                  <${Loading} />
                              </div>`
                            : html`<${CodeEditor}
                                  path=${currentFile.path}
                                  value=${currentFile.content}
                                  readOnly
                                  markers=${[]}
                              />`
                    }
                </div>
            </div>

            <${Console}
                logs=${state.logs}
                chatInput=${chatInput}
                setChatInput=${setChatInput}
                handleChatSubmit=${handleChatSubmit}
                connectionState=${state.connectionState}
                logsRef=${logsRef}
            />
        </div>
    `;
}

const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(html`<${App} />`);
