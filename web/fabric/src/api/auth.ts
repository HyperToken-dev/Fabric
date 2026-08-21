import type { CurrentUser as ProtoCurrentUser } from '../gen/auth_pb';
import { authClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString, safeInteger } from '../rpc/values';

export type CurrentUser = {
    userId: number;
    email: string;
    displayName: string;
    avatarUrl: string;
    role: string;
    oauthEnabled: boolean;
};

function toCurrentUser(user: ProtoCurrentUser | undefined): CurrentUser {
    if (!user) throw new Error('Invalid response field: user');
    return {
        userId: safeInteger(user.userId, 'user.userId', false),
        email: requireString(user.email, 'user.email'),
        displayName: requireString(user.displayName, 'user.displayName', true),
        avatarUrl: requireString(user.avatarUrl, 'user.avatarUrl', true),
        role: requireString(user.role, 'user.role'),
        oauthEnabled: user.oauthEnabled,
    };
}

export async function getCurrentUser(signal?: AbortSignal): Promise<CurrentUser> {
    const response = await callAdminRpc(() => authClient.getCurrentUser({}, { signal }));
    return toCurrentUser(response.user);
}

export async function logout(oauthEnabled: boolean): Promise<void> {
    await callAdminRpc(() => authClient.logout({}));
    window.location.href = oauthEnabled ? '/auth/login' : '/';
}
