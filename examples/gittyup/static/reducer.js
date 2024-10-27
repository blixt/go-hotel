import { enableMapSet, produce } from "immer";

enableMapSet();

/** @typedef {"disconnected" | "connecting" | "waitingForInit" | "ready"} ConnectionState */

/**
 * @typedef {Object} ConnectingAction
 * @property {"CONNECTING"} type
 * @property {string} repoURL
 * @property {WebSocket} socket
 */

/**
 * @typedef {Object} InitializeAction
 * @property {"INITIALIZE"} type
 * @property {number} currentUserId
 * @property {import("./messages").UserMetadata[]} users
 * @property {string[]} files
 * @property {string} repoHash
 * @property {string} commit
 */

/**
 * @typedef {Object} DisconnectedAction
 * @property {"DISCONNECTED"} type
 * @property {string|null} error
 */

/**
 * @typedef {Object} LogAction
 * @property {"LOG"} type
 * @property {string} message
 */

/**
 * @typedef {Object} SelectFileAction
 * @property {"SELECT_FILE"} type
 * @property {string} path
 */

/**
 * @typedef {Object} RepoStateAction
 * @property {"REPO_STATE"} type
 * @property {string[]} files
 * @property {string} repoHash
 * @property {string} commit
 */

/**
 * @typedef {Object} UserStateAction
 * @property {"USER_STATE"} type
 * @property {number} id
 * @property {string} name
 */

/**
 * @typedef {Object} UserJoinedAction
 * @property {"USER_JOINED"} type
 * @property {UserMetadata} user
 */

/**
 * @typedef {Object} UserLeftAction
 * @property {"USER_LEFT"} type
 * @property {number} id
 */

/**
 * @typedef {Object} ChatMessageAction
 * @property {"CHAT_MESSAGE"} type
 * @property {number} userId
 * @property {string} content
 */

/** @typedef {ConnectingAction | InitializeAction | DisconnectedAction | LogAction | SelectFileAction | RepoStateAction | UserStateAction | UserJoinedAction | UserLeftAction | ChatMessageAction} Action */

/**
 * @typedef {import("./messages").UserMetadata} UserMetadata
 */

/**
 * @typedef {Object} State
 * @property {ConnectionState} connectionState
 * @property {string[]} logs
 * @property {string[]} files
 * @property {string|null} selectedFile
 * @property {string|null} repoHash
 * @property {string|null} repoURL
 * @property {string|null} currentCommit
 * @property {string|null} error
 * @property {UserMetadata|null} user
 * @property {Map<number, UserMetadata>} users
 * @property {WebSocket|null} socket
 */

/** @type {State} */
export const initialState = {
    connectionState: "disconnected",
    logs: [],
    files: [],
    selectedFile: null,
    repoHash: null,
    repoURL: null,
    currentCommit: null,
    error: null,
    user: null,
    users: new Map(),
    socket: null,
};

/**
 * Reducer function to manage state transitions.
 * @param {State} state
 * @param {Action} action
 * @returns {State}
 */
export function reducer(state, action) {
    return produce(state, (/** @type {State} */ draft) => {
        switch (action.type) {
            case "CONNECTING":
                draft.connectionState = "connecting";
                draft.logs.push(`Connecting to ${action.repoURL}...`);
                draft.repoURL = action.repoURL;
                draft.error = null;
                draft.socket = action.socket;
                break;

            case "INITIALIZE": {
                draft.connectionState = "ready";
                draft.users = new Map(action.users.map((user) => [user.id, user]));
                const currentUser = draft.users.get(action.currentUserId);
                if (!currentUser) {
                    throw new Error(`Current user ${action.currentUserId} not found in users list`);
                }
                draft.user = currentUser;
                draft.files = action.files;
                draft.repoHash = action.repoHash;
                draft.currentCommit = action.commit;
                draft.logs.push(`Connected to repository at commit ${action.commit}`);
                draft.error = null;
                break;
            }

            case "DISCONNECTED":
                draft.connectionState = "disconnected";
                draft.error = action.error;
                draft.files = [];
                draft.selectedFile = null;
                draft.repoHash = null;
                draft.repoURL = null;
                draft.currentCommit = null;
                draft.user = null;
                draft.users = new Map();
                draft.socket = null;
                break;

            case "LOG":
                draft.logs.push(action.message);
                break;

            case "SELECT_FILE":
                draft.selectedFile = action.path;
                break;

            case "REPO_STATE":
                draft.files = action.files;
                draft.repoHash = action.repoHash;
                draft.currentCommit = action.commit;
                // Clear selected file if it's no longer in the files list
                if (!action.files.includes(draft.selectedFile)) {
                    draft.selectedFile = null;
                }
                break;

            case "USER_STATE":
                draft.users.set(action.id, { id: action.id, name: action.name });
                draft.user = { id: action.id, name: action.name };
                break;

            case "USER_JOINED":
                if (draft.users.has(action.user.id)) {
                    console.error(`User ${action.user.id} already exists in users list`);
                    return;
                }
                draft.users.set(action.user.id, action.user);
                draft.logs.push(`${action.user.name} (id: ${action.user.id}) joined the room`);
                break;

            case "USER_LEFT": {
                const leavingUser = draft.users.get(action.id);
                if (!leavingUser) {
                    console.error(`User ${action.id} not found in users list`);
                    return;
                }
                draft.users.delete(action.id);
                draft.logs.push(`${leavingUser.name} (id: ${leavingUser.id}) left the room`);
                break;
            }

            case "CHAT_MESSAGE": {
                const user = draft.users.get(action.userId);
                if (!user) {
                    console.error(`User ${action.userId} not found in users list`);
                    return;
                }
                draft.logs.push(`<${user.name}> ${action.content}`);
                break;
            }
        }
    });
}
