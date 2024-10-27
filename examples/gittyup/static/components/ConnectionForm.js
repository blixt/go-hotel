import htm from "htm";
import React, { useState } from "react";

const html = htm.bind(React.createElement);

const REPO_URL_PATTERN = "^(https?://|git@).*\\.git$|^[\\w\\-\\.]+/[\\w\\-\\.]+/[\\w\\-\\.]+$";
const NAME_PATTERN = "[A-Za-z0-9\\s\\-_]+";

/**
 * @typedef {import('../reducer.js').State} State
 * @typedef {import('../reducer.js').Action} Action
 */

/**
 * @typedef {Object} ConnectionFormProps
 * @property {State} state - The current application state
 * @property {(action: Action) => void} dispatch - The dispatch function to update state
 */

const getQueryParams = () => {
    const params = new URLSearchParams(window.location.search);
    return {
        repoURL: params.get("repo") || "github.com/blixt/chrome-ai-game",
        name: params.get("name") || "Bob",
    };
};

const updateQueryParams = (repoURL, name) => {
    const params = new URLSearchParams();
    params.set("repo", repoURL);
    params.set("name", name);
    window.history.replaceState({}, "", `?${params.toString()}`);
};

/**
 * Converts a repository URL or import path to a standardized format.
 * @param {string} originalInput
 * @returns {string}
 */
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

/**
 * Form component for handling repository connections
 * @param {ConnectionFormProps} props
 * @returns {import('react').ReactElement}
 */
export function ConnectionForm({ state, dispatch }) {
    const [defaultValues] = useState(() => getQueryParams());

    const handleSubmit = async (e) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        const repoURL = convertToImportPath(formData.get("repoURL").trim());
        const name = formData.get("name").trim();

        updateQueryParams(repoURL, name);

        const socketURL = `ws://localhost:8080/v1/repo/${repoURL}?name=${encodeURIComponent(name)}`;
        dispatch({ type: "CONNECTING", repoURL, socket: new WebSocket(socketURL) });
    };

    const handleDisconnect = () => {
        if (state.socket) {
            state.socket.close();
            dispatch({ type: "DISCONNECTED", error: null });
        }
    };

    return html`
        <div className="p-4 border-b border-slate-300 dark:border-slate-700">
            <h1 className="text-2xl font-bold mb-4 text-slate-800 dark:text-slate-200">GittyUp</h1>
            <form onSubmit=${handleSubmit} className="space-y-4">
                <div>
                    <input
                        type="text"
                        name="repoURL"
                        placeholder="Enter Git repository URL or Go import path"
                        defaultValue=${defaultValues.repoURL}
                        required
                        pattern=${REPO_URL_PATTERN}
                        title="Please enter a valid Git repository URL (https:// or git@) or Go import path (e.g. github.com/user/repo)"
                        disabled=${state.connectionState !== "disconnected"}
                        className="w-full px-3 py-2 border border-slate-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-slate-700 dark:border-slate-600 dark:text-white disabled:bg-slate-100 dark:disabled:bg-slate-800"
                    />
                </div>
                <div>
                    <input
                        type="text"
                        name="name"
                        placeholder="Enter your name"
                        defaultValue=${defaultValues.name}
                        required
                        minLength="2"
                        maxLength="50"
                        pattern=${NAME_PATTERN}
                        disabled=${state.connectionState !== "disconnected"}
                        className="w-full px-3 py-2 border border-slate-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-slate-700 dark:border-slate-600 dark:text-white disabled:bg-slate-100 dark:disabled:bg-slate-800"
                    />
                </div>
                <div className="flex gap-2">
                    <button
                        type="submit"
                        disabled=${state.connectionState !== "disconnected"}
                        className="flex-1 px-4 py-2 bg-blue-500 dark:bg-blue-700 text-white rounded-md hover:bg-blue-600 dark:hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:bg-slate-200 dark:disabled:bg-slate-600 disabled:text-slate-500 dark:disabled:text-slate-400 disabled:cursor-not-allowed transition-colors"
                    >
                        ${state.connectionState === "connecting" ? "Connecting..." : "Connect"}
                    </button>
                    <button
                        type="button"
                        onClick=${handleDisconnect}
                        disabled=${state.connectionState === "disconnected"}
                        className="flex-1 px-4 py-2 bg-rose-500 dark:bg-rose-700 text-white rounded-md hover:bg-rose-600 dark:hover:bg-rose-600 focus:outline-none focus:ring-2 focus:ring-rose-500 focus:ring-offset-2 disabled:bg-slate-200 dark:disabled:bg-slate-600 disabled:text-slate-500 dark:disabled:text-slate-400 disabled:cursor-not-allowed transition-colors"
                    >
                        Disconnect
                    </button>
                </div>
            </form>
        </div>
    `;
}
