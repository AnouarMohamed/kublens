import { useEffect } from "react";
import { VIEW_NAVIGATE_EVENT, consumePendingViewNavigationDetail, type ViewNavigateDetail } from "../../app/viewNavigation";
import { intentOptions } from "./constants";
import { OpsAssistantComposer } from "./components/OpsAssistantComposer";
import { OpsAssistantHeader } from "./components/OpsAssistantHeader";
import { OpsAssistantIntentBar } from "./components/OpsAssistantIntentBar";
import { OpsAssistantMessages } from "./components/OpsAssistantMessages";
import { OpsAssistantPromptSections } from "./components/OpsAssistantPromptSections";
import { OpsAssistantSidebar } from "./components/OpsAssistantSidebar";
import { useOpsAssistantView } from "./hooks/useOpsAssistantView";

export default function OpsAssistant() {
  const {
    messages,
    isLoading,
    lastAssistant,
    suggestionPool,
    diagnosticPrompts,
    sessions,
    activeSessionId,
    input,
    intentMode,
    copiedMessageID,
    referenceFeedback,
    namespaces,
    selectedNamespace,
    scrollRef,
    quickActions,
    decisionPrompts,
    assistantReplies,
    setInput,
    setIntentMode,
    setSelectedNamespace,
    startNewSession,
    selectSession,
    deleteSession,
    submit,
    copyMessage,
    submitReferenceFeedback,
    resetSession,
  } = useOpsAssistantView();

  useEffect(() => {
    const applyPrefill = (detail?: ViewNavigateDetail | null) => {
      const prefillMessage = detail?.view === "assistant" ? detail.prefillMessage?.trim() : "";
      if (!prefillMessage) {
        return;
      }
      setInput(prefillMessage);
    };

    applyPrefill(consumePendingViewNavigationDetail());

    const onNavigate = (event: Event) => {
      const custom = event as CustomEvent<ViewNavigateDetail>;
      applyPrefill(custom.detail);
    };

    window.addEventListener(VIEW_NAVIGATE_EVENT, onNavigate as EventListener);
    return () => window.removeEventListener(VIEW_NAVIGATE_EVENT, onNavigate as EventListener);
  }, [setInput]);

  return (
    <div className="h-[calc(100vh-140px)] app-shell overflow-hidden grid grid-cols-1 xl:grid-cols-[1fr_340px]">
      <section className="flex flex-col overflow-hidden border-r border-zinc-700">
        <OpsAssistantHeader
          intentMode={intentMode}
          messageCount={messages.length}
          selectedNamespace={selectedNamespace}
          namespaces={namespaces}
          onNamespaceChange={setSelectedNamespace}
          onClear={resetSession}
        />

        <OpsAssistantIntentBar intentMode={intentMode} intentOptions={intentOptions} onIntentChange={setIntentMode} />

        <OpsAssistantPromptSections
          suggestionPool={suggestionPool}
          diagnosticPrompts={diagnosticPrompts}
          onRunPrompt={(prompt) => void submit(prompt)}
        />

        <OpsAssistantMessages
          messages={messages}
          isLoading={isLoading}
          copiedMessageID={copiedMessageID}
          referenceFeedback={referenceFeedback}
          scrollRef={scrollRef}
          onCopy={copyMessage}
          onRunPrompt={(prompt) => void submit(prompt)}
          onReferenceFeedback={submitReferenceFeedback}
        />

        <OpsAssistantComposer
          input={input}
          intentMode={intentMode}
          isLoading={isLoading}
          onInputChange={setInput}
          onSubmit={() => void submit()}
        />
      </section>

      <OpsAssistantSidebar
        sessions={sessions}
        activeSessionId={activeSessionId}
        quickActions={quickActions}
        decisionPrompts={decisionPrompts}
        assistantReplies={assistantReplies}
        isLoading={isLoading}
        referencesCount={lastAssistant?.references?.length ?? 0}
        selectedNamespace={selectedNamespace}
        latestResources={lastAssistant?.resources ?? []}
        onStartNewSession={startNewSession}
        onSelectSession={selectSession}
        onDeleteSession={deleteSession}
        onRunPrompt={(prompt) => void submit(prompt)}
      />
    </div>
  );
}
