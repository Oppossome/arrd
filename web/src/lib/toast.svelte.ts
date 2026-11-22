// Notification state. The UI lives in the ToastHost organism.

export type ToastKind = 'info' | 'success' | 'error';

export interface ToastMessage {
	id: number;
	kind: ToastKind;
	text: string;
}

let nextId = 1;

class Toasts {
	items = $state<ToastMessage[]>([]);

	push(kind: ToastKind, text: string, timeoutMs = 5000): number {
		const id = nextId++;
		this.items.push({ id, kind, text });
		if (timeoutMs > 0) setTimeout(() => this.dismiss(id), timeoutMs);
		return id;
	}

	dismiss(id: number): void {
		this.items = this.items.filter((t) => t.id !== id);
	}
}

export const toasts = new Toasts();
