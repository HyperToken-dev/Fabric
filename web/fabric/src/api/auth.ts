import type { CurrentUser as ProtoCurrentUser } from '../gen/auth_pb';
import { authClient } from '../rpc/clients';
import { callAdminRpc } from '../rpc/errors';
import { requireString } from '../rpc/values';

export type CurrentUser = {
    openid: string;
    email: string;
    displayName: string;
    avatarUrl: string;
    role: string;
    permissions: string[];
    oauthEnabled: boolean;
};

export function isAdminUser(user: CurrentUser | null | undefined): boolean {
    return user?.permissions.includes('fabric_admin') ?? false;
}

function toCurrentUser(user: ProtoCurrentUser | undefined): CurrentUser {
    if (!user) throw new Error('Invalid response field: user');
    return {
        openid: requireString(user.openid, 'user.openid'),
        email: requireString(user.email, 'user.email'),
        displayName: requireString(user.displayName, 'user.displayName', true),
        avatarUrl: requireString(user.avatarUrl, 'user.avatarUrl', true),
        role: requireString(user.role, 'user.role'),
        permissions: user.permissions.map((permission, index) =>
            requireString(permission, `user.permissions[${index}]`),
        ),
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
