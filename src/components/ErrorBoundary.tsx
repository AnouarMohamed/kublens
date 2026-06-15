import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

/**
 * Standard React class ErrorBoundary component to prevent application-wide crashes
 * when individual views fail to render or experience runtime errors.
 */
export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Uncaught view rendering error:", error, errorInfo);
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="surface border-red-500/35 bg-zinc-950 p-6 text-zinc-100 space-y-4">
          <h2 className="text-sm font-semibold text-red-500">View Render Failed</h2>
          <p className="text-xs text-zinc-400 font-mono leading-relaxed max-w-xl">
            An error occurred while rendering this view. This can happen when the cluster returns unexpected resource
            states.
          </p>
          {this.state.error && (
            <pre className="p-3 bg-zinc-900 border border-zinc-800 rounded text-[11px] text-zinc-400 overflow-x-auto max-w-xl">
              {this.state.error.name}: {this.state.error.message}
            </pre>
          )}
          <button
            type="button"
            onClick={() => this.setState({ hasError: false, error: null })}
            className="btn-sm bg-zinc-800 hover:bg-zinc-700"
          >
            Retry View
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
