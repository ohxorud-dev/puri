export interface Step {
  id: string;
  name: string;
  desc: string;
  tags: string[];
}

export const steps: Step[] = [
  {
    id: "basic",
    name: "기초 다지기",
    desc: "기초적인 구문 규칙, 변수, 배열 등 가장 기본적인 자료구조와 알고리즘 문법을 먼저 익히는 입문 단계입니다.",
    tags: ["입출력", "분기문", "반복문", "1차원 배열"],
  },
  {
    id: "math",
    name: "수학적 사고",
    desc: "수학적 개념과 수식을 활용하여 문제를 분석하고 해결하는 능력을 기릅니다.",
    tags: ["사칙연산", "약수와 배수", "소수", "조합론"],
  },
  {
    id: "string",
    name: "문자열 처리",
    desc: "문자열 탐색, 변환, 파싱 등 문자열을 다루는 다양한 기법을 학습합니다.",
    tags: ["문자열", "정규표현식", "파싱", "아스키코드"],
  },
  {
    id: "ds-basic",
    name: "기본 자료구조",
    desc: "스택, 큐, 덱 등 기본적인 자료구조의 원리를 이해하고 활용합니다.",
    tags: ["스택", "큐", "덱", "연결 리스트"],
  },
  {
    id: "sorting",
    name: "정렬과 탐색",
    desc: "다양한 정렬 알고리즘과 이분 탐색의 원리를 이해하고 적용합니다.",
    tags: ["버블 정렬", "병합 정렬", "이분 탐색", "투 포인터"],
  },
  {
    id: "bruteforce",
    name: "완전 탐색",
    desc: "가능한 모든 경우를 탐색하여 해를 구하는 전략을 학습합니다.",
    tags: ["브루트포스", "백트래킹", "비트마스크", "순열"],
  },
  {
    id: "dp",
    name: "동적 계획법",
    desc: "부분 문제의 최적해를 이용하여 전체 문제의 최적해를 구하는 핵심 알고리즘입니다.",
    tags: ["메모이제이션", "탑다운", "바텀업", "배낭 문제"],
  },
  {
    id: "greedy",
    name: "그리디 알고리즘",
    desc: "매 순간 최적의 선택을 통해 전체 최적해를 구하는 전략을 학습합니다.",
    tags: ["탐욕법", "활동 선택", "최소 스패닝", "구간 스케줄링"],
  },
  {
    id: "graph",
    name: "그래프 탐색",
    desc: "BFS, DFS를 기반으로 그래프 구조를 탐색하고 응용하는 방법을 배웁니다.",
    tags: ["BFS", "DFS", "위상 정렬", "연결 요소"],
  },
  {
    id: "shortest-path",
    name: "최단 경로",
    desc: "다익스트라, 벨만-포드 등 최단 경로 알고리즘을 학습합니다.",
    tags: ["다익스트라", "벨만-포드", "플로이드-워셜", "SPFA"],
  },
  {
    id: "tree",
    name: "트리",
    desc: "트리 구조의 성질을 이해하고 다양한 트리 알고리즘을 학습합니다.",
    tags: ["이진 트리", "LCA", "트리 DP", "오일러 경로"],
  },
  {
    id: "ds-advanced",
    name: "고급 자료구조",
    desc: "세그먼트 트리, 펜윅 트리 등 고급 자료구조를 활용한 문제를 해결합니다.",
    tags: ["세그먼트 트리", "펜윅 트리", "유니온 파인드", "해시"],
  },
];

export const stepMap: Record<string, Step> = Object.fromEntries(
  steps.map(s => [s.id, s])
);

export function getStepName(id: string): string {
  return stepMap[id]?.name ?? id;
}
