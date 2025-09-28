(function (global) {
    const state = {
        activeSession: null,
        lastExpansionResult: null,
    };

    const listeners = {
        activeSession: new Set(),
        lastExpansionResult: new Set(),
    };

    function notify(key, value) {
        listeners[key].forEach((listener) => {
            try {
                listener(value);
            } catch (error) {
                console.error(`State listener for ${key} failed:`, error);
            }
        });
    }

    function createSetter(key) {
        return function setValue(value) {
            if (state[key] === value) {
                return value;
            }
            state[key] = value ?? null;
            notify(key, state[key]);
            return state[key];
        };
    }

    function createGetter(key) {
        return function getValue() {
            return state[key];
        };
    }

    function createSubscribe(key) {
        return function subscribe(listener) {
            if (typeof listener !== 'function') return () => {};
            listeners[key].add(listener);
            return () => listeners[key].delete(listener);
        };
    }

    function reset() {
        setActiveSession(null);
        setLastExpansionResult(null);
    }

    const setActiveSession = createSetter('activeSession');
    const setLastExpansionResult = createSetter('lastExpansionResult');

    const appState = {
        getActiveSession: createGetter('activeSession'),
        setActiveSession,
        onActiveSessionChange: createSubscribe('activeSession'),
        getLastExpansionResult: createGetter('lastExpansionResult'),
        setLastExpansionResult,
        onLastExpansionResultChange: createSubscribe('lastExpansionResult'),
        reset,
    };

    global.AppState = appState;
})(window);
