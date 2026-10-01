import {
  AlertTriangle,
  ArrowLeft,
  Check,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Flag,
  Play,
  RotateCcw,
  Server,
  Square,
  Target,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Button } from "../../components/ui/Button";
import { ConfirmDialog } from "../../components/ui/ConfirmDialog";
import { ErrorState, LoadingSkeleton } from "../../components/ui/Feedback";
import { ProgressBar } from "../../components/ui/ProgressBar";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useToast } from "../../components/ui/Toast";
import { formatDateTime, lifecycleTone } from "../../lib/format";
import { platformRepository } from "../../services/repositories";
import type {
  AssessmentDefinition,
  StudentAssessment,
} from "../../types/domain";

export function AssessmentWorkspacePage() {
  const { assessmentId } = useParams();
  const navigate = useNavigate();
  const { notify } = useToast();
  const [attempt, setAttempt] = useState<StudentAssessment | null>(null);
  const [definition, setDefinition] = useState<AssessmentDefinition | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [questionIndex, setQuestionIndex] = useState(0);
  const [draft, setDraft] = useState("");
  const [feedback, setFeedback] = useState<Record<string, string>>({});
  const [finishOpen, setFinishOpen] = useState(false);
  useEffect(() => {
    Promise.all([
      platformRepository.listStudentAssessments(),
      platformRepository.listAssessmentDefinitions(),
    ])
      .then(([attempts, definitions]) => {
        const found = attempts.find((item) => item.id === assessmentId);
        if (!found) throw new Error("Attempt not found");
        setAttempt(found);
        setDefinition(
          definitions.find((item) => item.id === found.definitionId) ?? null,
        );
      })
      .catch(() => setError("This assessment record could not be loaded."))
      .finally(() => setLoading(false));
  }, [assessmentId]);
  const questions = definition?.questions ?? [];
  const question = questions[questionIndex];
  const answered = Object.keys(attempt?.answers ?? {}).length;
  const progress = questions.length
    ? Math.round((answered / questions.length) * 100)
    : (attempt?.progress ?? 0);
  const timer = useMemo(
    () =>
      `${String(Math.floor((attempt?.duration ?? 0) / 60)).padStart(2, "0")}:${String((attempt?.duration ?? 0) % 60).padStart(2, "0")}:00`,
    [attempt?.duration],
  );
  if (loading) return <LoadingSkeleton rows={6} />;
  if (error || !attempt)
    return <ErrorState message={error || "Assessment not found."} />;
  if (attempt.status !== "Active")
    return (
      <div className="space-y-5">
        <Link
          to="/student/assessments"
          className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to assessments
        </Link>
        <section className="surface p-6">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge tone={lifecycleTone(attempt.status)}>
              {attempt.status}
            </StatusBadge>
            <span className="text-sm text-secondary">{attempt.level}</span>
          </div>
          <h2 className="page-heading mt-4">{attempt.name}</h2>
          <dl className="mt-6 grid gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-sm text-secondary">Scheduled</dt>
              <dd className="mt-1 font-medium">
                {formatDateTime(attempt.scheduledAt)}
              </dd>
            </div>
            <div>
              <dt className="text-sm text-secondary">Duration</dt>
              <dd className="mt-1 font-medium">{attempt.duration} minutes</dd>
            </div>
            <div>
              <dt className="text-sm text-secondary">Attempt</dt>
              <dd className="mt-1 font-medium">
                {attempt.attempt || "Not started"}
              </dd>
            </div>
          </dl>
          <p className="mt-6 rounded-lg bg-app p-4 text-sm text-secondary">
            {attempt.eligibility}
          </p>
        </section>
      </div>
    );
  const setEnvironment = async (
    state: StudentAssessment["environmentState"],
  ) => {
    const updated = await platformRepository.updateStudentAssessment(
      attempt.id,
      { environmentState: state },
    );
    setAttempt(updated);
    notify(
      `Lab environment ${state === "Running" ? "started" : "stopped"} (simulated).`,
    );
  };
  const submit = async () => {
    if (!question || !draft.trim()) {
      setFeedback((value) => ({
        ...value,
        [question?.id ?? ""]: "Enter an answer before submitting.",
      }));
      return;
    }
    const correct =
      draft.trim().toLowerCase() === question.expectedAnswer.toLowerCase();
    const answers = { ...(attempt.answers ?? {}), [question.id]: draft.trim() };
    const updated = await platformRepository.updateStudentAssessment(
      attempt.id,
      {
        answers,
        progress: Math.round(
          (Object.keys(answers).length / questions.length) * 100,
        ),
      },
    );
    setAttempt(updated);
    setFeedback((value) => ({
      ...value,
      [question.id]: correct
        ? "Accepted in this mock assessment."
        : "Submitted. Review the prompt and try again.",
    }));
    notify("Answer submitted.");
  };
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link
          to="/student/assessments"
          className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to assessments
        </Link>
        <div className="flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-900">
          <AlertTriangle className="h-4 w-4" />
          Authorized college testing only
        </div>
      </div>
      <section className="surface overflow-hidden">
        <header className="grid gap-4 border-b border-border p-4 sm:p-5 lg:grid-cols-[1fr_auto_auto] lg:items-center">
          <div>
            <div className="flex items-center gap-2">
              <StatusBadge tone="info">Active</StatusBadge>
              <span className="text-sm text-secondary">{attempt.level}</span>
            </div>
            <h2 className="mt-2 text-xl font-semibold text-strong">
              {attempt.name}
            </h2>
          </div>
          <div className="rounded-lg border border-border bg-app px-4 py-2 text-center">
            <div className="text-xs font-semibold uppercase tracking-wide text-secondary">
              Time remaining
            </div>
            <div className="mt-1 flex items-center justify-center gap-2 font-mono text-lg font-semibold text-strong">
              <Clock3 className="h-4 w-4 text-primary" />
              {timer}
            </div>
          </div>
          <Button variant="danger" onClick={() => setFinishOpen(true)}>
            Finish assessment
          </Button>
        </header>
        <div className="grid lg:grid-cols-[250px_minmax(0,1fr)_290px]">
          <aside className="border-b border-border bg-app p-4 lg:border-b-0 lg:border-r">
            <h3 className="text-sm font-semibold text-strong">Questions</h3>
            <div className="mt-3 grid grid-cols-5 gap-2 lg:grid-cols-3">
              {questions.map((item, index) => (
                <button
                  key={item.id}
                  onClick={() => {
                    setQuestionIndex(index);
                    setDraft(attempt.answers?.[item.id] ?? "");
                  }}
                  className={`grid h-10 place-items-center rounded-lg border text-sm font-semibold ${index === questionIndex ? "border-primary bg-primary text-white" : attempt.answers?.[item.id] ? "border-emerald-200 bg-emerald-50 text-success" : "border-border bg-white text-secondary hover:bg-subtle"}`}
                  aria-label={`Question ${index + 1}${attempt.answers?.[item.id] ? ", answered" : ""}`}
                >
                  {attempt.answers?.[item.id] ? (
                    <Check className="h-4 w-4" />
                  ) : (
                    index + 1
                  )}
                </button>
              ))}
            </div>
            <div className="mt-5">
              <ProgressBar value={progress} label="Overall completion" />
            </div>
          </aside>
          <main className="min-h-[440px] p-5 sm:p-7">
            {question ? (
              <>
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm font-semibold text-primary">
                    Question {questionIndex + 1} of {questions.length}
                  </p>
                  <span className="text-sm font-medium text-secondary">
                    {question.points} points
                  </span>
                </div>
                <h3 className="mt-4 text-lg font-semibold leading-7 text-strong">
                  {question.prompt}
                </h3>
                {question.hint && (
                  <details className="mt-4 rounded-lg border border-border bg-app p-3">
                    <summary className="cursor-pointer text-sm font-semibold text-strong">
                      Optional hint
                    </summary>
                    <p className="mt-2 text-sm text-secondary">
                      {question.hint}
                    </p>
                  </details>
                )}
                <label className="mt-6 block text-sm font-semibold text-strong">
                  {question.type === "flag"
                    ? "Flag answer"
                    : question.type === "multiple-choice"
                      ? "Choose an answer"
                      : "Short answer"}
                  {question.type === "multiple-choice" ? (
                    <select
                      className="input mt-2"
                      value={draft}
                      onChange={(e) => setDraft(e.target.value)}
                    >
                      <option value="">Select one</option>
                      {question.options?.map((option) => (
                        <option key={option}>{option}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      className="input mt-2 font-mono"
                      value={draft}
                      onChange={(e) => setDraft(e.target.value)}
                      placeholder={
                        question.type === "flag"
                          ? "CSPACE{...}"
                          : "Enter your answer"
                      }
                    />
                  )}
                </label>
                {feedback[question.id] && (
                  <p
                    role="status"
                    className={`mt-2 text-sm font-medium ${feedback[question.id].startsWith("Accepted") ? "text-success" : "text-warning"}`}
                  >
                    {feedback[question.id]}
                  </p>
                )}
                <div className="mt-6 flex flex-wrap items-center justify-between gap-3">
                  <Button
                    variant="secondary"
                    disabled={questionIndex === 0}
                    onClick={() => {
                      const next = questionIndex - 1;
                      setQuestionIndex(next);
                      setDraft(attempt.answers?.[questions[next].id] ?? "");
                    }}
                  >
                    <ChevronLeft className="h-4 w-4" />
                    Previous
                  </Button>
                  <div className="flex gap-2">
                    <Button onClick={submit}>
                      <Flag className="h-4 w-4" />
                      Submit answer
                    </Button>
                    <Button
                      variant="secondary"
                      disabled={questionIndex === questions.length - 1}
                      onClick={() => {
                        const next = questionIndex + 1;
                        setQuestionIndex(next);
                        setDraft(attempt.answers?.[questions[next].id] ?? "");
                      }}
                    >
                      Next
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </>
            ) : (
              <div className="grid h-full place-items-center text-center">
                <div>
                  <Target className="mx-auto h-8 w-8 text-muted" />
                  <h3 className="mt-3 font-semibold">
                    No questions configured
                  </h3>
                  <p className="mt-1 text-sm text-secondary">
                    This mock definition has no questions yet.
                  </p>
                </div>
              </div>
            )}
          </main>
          <aside className="border-t border-border bg-app p-4 lg:border-l lg:border-t-0">
            <h3 className="flex items-center gap-2 text-sm font-semibold text-strong">
              <Server className="h-4 w-4" />
              Lab environment
            </h3>
            <div className="mt-4 rounded-lg border border-border bg-white p-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-secondary">Status</span>
                <StatusBadge
                  tone={
                    attempt.environmentState === "Running"
                      ? "success"
                      : "neutral"
                  }
                >
                  {attempt.environmentState ?? "Stopped"}
                </StatusBadge>
              </div>
              <div className="mt-4 border-t border-border pt-4">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted">
                  Assigned target
                </p>
                <p className="mt-1 font-mono text-sm font-semibold text-strong">
                  {attempt.environmentState === "Running"
                    ? attempt.targetIp
                    : "Available after start"}
                </p>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-2">
                {attempt.environmentState === "Running" ? (
                  <Button
                    variant="secondary"
                    onClick={() => setEnvironment("Stopped")}
                  >
                    <Square className="h-4 w-4" />
                    Stop
                  </Button>
                ) : (
                  <Button onClick={() => setEnvironment("Running")}>
                    <Play className="h-4 w-4" />
                    Start
                  </Button>
                )}
                <Button
                  variant="ghost"
                  onClick={() =>
                    notify("Environment reset requested (simulated).")
                  }
                >
                  <RotateCcw className="h-4 w-4" />
                  Reset
                </Button>
              </div>
            </div>
            <p className="mt-3 text-xs leading-5 text-secondary">
              Provisioning and network isolation are represented for interface
              testing only. No Docker action is executed.
            </p>
          </aside>
        </div>
      </section>
      <ConfirmDialog
        open={finishOpen}
        title="Finish this assessment?"
        description={`You have answered ${answered} of ${questions.length} questions. Finishing will stop the mock environment and mark this attempt completed.`}
        confirmLabel="Finish assessment"
        tone="danger"
        onCancel={() => setFinishOpen(false)}
        onConfirm={async () => {
          await platformRepository.updateStudentAssessment(attempt.id, {
            status: "Completed",
            environmentState: "Stopped",
            progress,
          });
          notify("Assessment finished and environment stopped.");
          navigate("/student/assessments");
        }}
      />
    </div>
  );
}
