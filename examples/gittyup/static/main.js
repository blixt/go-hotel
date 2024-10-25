import htm from "htm";
import React, { useState, useEffect, useRef } from "react";
import ReactDOM from "react-dom/client";

const html = htm.bind(React.createElement);

// Define regex patterns outside of template string
const REPO_URL_PATTERN = "^(https?://|git@).*\\.git$|^[\\w\\-\\.]+/[\\w\\-\\.]+/[\\w\\-\\.]+$";
const NAME_PATTERN = "[A-Za-z0-9\\s\\-_]+";

function App() {
    const [isConnected, setIsConnected] = useState(false);
    const [logs, setLogs] = useState([]);
    const logsRef = useRef(null);
    const socketRef = useRef(null);

    const convertToImportPath = (originalInput) => {
        if (/^[\w\-\.]+\/[\w\-\.]+\/[\w\-\.]+$/.test(originalInput)) {
            return originalInput;
        }

        let result = originalInput;

        if (result.startsWith("https://") || result.startsWith("http://")) {
            result = result.replace(/^https?:\/\//, "").replace(/\.git$/, "");
            return result;
        }

        if (result.startsWith("git@")) {
            result = result
                .replace(/^git@/, "")
                .replace(":", "/")
                .replace(/\.git$/, "");
            return result;
        }

        return result;
    };

    const handleSubmit = async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        const repoUrl = convertToImportPath(formData.get("repoUrl").trim());
        const name = formData.get("name").trim();

        const wsUrl = `ws://localhost:8080/v1/repo/${repoUrl}?name=${encodeURIComponent(name)}`;
        socketRef.current = new WebSocket(wsUrl);

        socketRef.current.onopen = () => {
            setIsConnected(true);
            addLog("Connected to WebSocket.");
        };

        socketRef.current.onmessage = (event) => {
            addLog(event.data);
        };

        socketRef.current.onclose = () => {
            setIsConnected(false);
            addLog("WebSocket connection closed.");
        };

        socketRef.current.onerror = (error) => {
            addLog(`WebSocket error: ${error.message}`);
        };
    };

    const addLog = (message) => {
        setLogs((prev) => [...prev, message]);
    };

    useEffect(() => {
        if (logsRef.current) {
            logsRef.current.scrollTop = logsRef.current.scrollHeight;
        }
    }, []);

    return html`
        <div className="max-w-2xl mx-auto p-6">
            <h1 className="text-3xl font-bold mb-8 text-gray-800 dark:text-gray-200">GittyUp</h1>

            <form onSubmit=${handleSubmit} className="space-y-4 mb-6">
                <div>
                    <input
                        type="text"
                        name="repoUrl"
                        placeholder="Enter Git repository URL or Go import path"
                        defaultValue="github.com/blixt/chrome-ai-game"
                        required
                        pattern=${REPO_URL_PATTERN}
                        title="Please enter a valid Git repository URL (https:// or git@) or Go import path (e.g. github.com/user/repo)"
                        disabled=${isConnected}
                        className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white disabled:bg-gray-100 dark:disabled:bg-gray-800"
                    />
                </div>
                <div>
                    <input
                        type="text"
                        name="name"
                        placeholder="Enter your name"
                        defaultValue="Bob"
                        required
                        minLength="2"
                        maxLength="50"
                        pattern=${NAME_PATTERN}
                        title="Name can contain letters, numbers, spaces, hyphens and underscores"
                        disabled=${isConnected}
                        className="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white disabled:bg-gray-100 dark:disabled:bg-gray-800"
                    />
                </div>
                <button
                    type="submit"
                    disabled=${isConnected}
                    className="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                >
                    Connect
                </button>
            </form>

            <div
                ref=${logsRef}
                className="h-[300px] border border-gray-300 rounded-md overflow-y-auto p-3 font-mono text-xs bg-gray-50 dark:bg-gray-800 dark:border-gray-700 dark:text-gray-200"
            >
				${logs.map((log, index) => html`<div key=${index} className="whitespace-pre-wrap break-words">${log}</div>`)}
            </div>
        </div>
    `;
}

const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(html`<${App} />`);
