const AUTH_RETURN_KEY = 'new-api:auth-return-to';

function safeCanvasReturnTo(value) {
  if (!value || typeof value !== 'string') return '';
  try {
    const target = new URL(value, window.location.origin);
    if (
      target.origin !== window.location.origin ||
      target.pathname !== '/canvas/authorize'
    ) {
      return '';
    }
    return `${target.pathname}${target.search}`;
  } catch {
    return '';
  }
}

export function rememberAuthReturnTo() {
  const params = new URLSearchParams(window.location.search);
  if (!params.has('return_to')) {
    if (window.location.pathname === '/login') {
      sessionStorage.removeItem(AUTH_RETURN_KEY);
    }
    return '';
  }
  const value = params.get('return_to');
  const target = safeCanvasReturnTo(value);
  if (target) sessionStorage.setItem(AUTH_RETURN_KEY, target);
  else sessionStorage.removeItem(AUTH_RETURN_KEY);
  return target;
}

export function getPostLoginPath(fallback = '/console') {
  const queryTarget = rememberAuthReturnTo();
  const storedTarget = safeCanvasReturnTo(
    sessionStorage.getItem(AUTH_RETURN_KEY),
  );
  return queryTarget || storedTarget || fallback;
}

export function clearAuthReturnTo() {
  sessionStorage.removeItem(AUTH_RETURN_KEY);
}
