import { useEffect, useState } from "react";

// Create a cache outside the hook to persist across renders
const fileContentCache = new Map();

/**
 * Custom hook to fetch and manage file content from a repository.
 *
 * @param {string|null} repoHash - The hash/identifier of the repository
 * @param {string|null} commit - The commit hash
 * @param {string|null} path - The file path within the repository
 * @returns {Object} An object containing:
 *   - path: The current file path
 *   - content: The file content as a string
 *   - isLoading: Boolean indicating whether the content is currently loading
 */
export function useFileContent(repoHash, commit, path) {
    const [state, setState] = useState({ path: "", content: "", isLoading: false });

    useEffect(() => {
        if (!path || !commit || !repoHash) {
            setState({ path: "", content: "", isLoading: false });
            return;
        }

        setState({ path, content: "", isLoading: true });

        const cacheKey = `${repoHash}/${commit}/${path}`;
        if (fileContentCache.has(cacheKey)) {
            setState({ path, content: fileContentCache.get(cacheKey), isLoading: false });
            return;
        }

        const controller = new AbortController();

        fetch(`/v1/file/${repoHash}/${commit}/${path}`, { signal: controller.signal })
            .then((response) => {
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                return response.text();
            })
            .then((newContent) => {
                fileContentCache.set(cacheKey, newContent);
                if (controller.signal.aborted) return;
                setState({ path, content: newContent, isLoading: false });
            })
            .catch((error) => {
                if (error.name === "AbortError") return;
                const errorContent = `// Error loading ${path}\n// ${error.message}`;
                setState({ path, content: errorContent, isLoading: false });
            });

        return () => controller.abort();
    }, [repoHash, path, commit]);

    return state;
}

export function clearFileContentCache() {
    fileContentCache.clear();
}
