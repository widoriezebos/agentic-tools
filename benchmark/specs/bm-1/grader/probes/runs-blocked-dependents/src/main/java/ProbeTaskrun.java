import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Properties;
import java.util.Set;
import java.util.regex.Pattern;

/** Shared source for the deliberately flawed BM-1 calibration probes. */
public final class ProbeTaskrun {
    private static final Pattern NAME = Pattern.compile("[A-Za-z0-9][A-Za-z0-9_.-]*");
    private static final String MODE = readMode();

    private ProbeTaskrun() {}

    public static void main(String[] args) {
        int code;
        try {
            code = execute(args);
        } catch (UsageOrConfig error) {
            System.err.println(error.getMessage());
            code = 2;
        } catch (RunnerFailure error) {
            System.err.println(error.getMessage());
            code = 3;
        } catch (Exception error) {
            System.err.println("probe runner failure: " + error.getMessage());
            code = 3;
        }
        System.exit(code);
    }

    private static int execute(String[] args) throws Exception {
        Options options = Options.parse(args);
        Path config = options.file.toAbsolutePath().normalize();
        Path base = config.getParent();
        if (base == null || !Files.isReadable(config)) {
            throw new UsageOrConfig("configuration file cannot be read: " + config);
        }
        Config parsed = Config.read(config, base);
        LinkedHashSet<String> selected = parsed.select(options.tasks);
        List<String> executionOrder = parsed.topological(selected);
        List<String> reportOrder = new ArrayList<>(selected);
        if (!MODE.equals("incomplete")) {
            reportOrder.sort(Comparator.naturalOrder());
        }
        adjustReportOrder(reportOrder, base);

        if (options.dryRun) {
            reportPlan(reportOrder, options.json);
            return 0;
        }

        Cache cache = Cache.open(base);
        Map<String, String> states = new HashMap<>();
        Map<String, String> identities = new HashMap<>();
        for (String name : executionOrder) {
            Task task = parsed.tasks.get(name);
            boolean blocked = false;
            for (String dep : task.deps) {
                String state = states.get(dep);
                if ("failed".equals(state) || "blocked".equals(state)) {
                    blocked = true;
                }
            }
            if (blocked && !MODE.equals("runs-blocked-dependents") && !MODE.equals("incomplete")) {
                states.put(name, "blocked");
                continue;
            }

            String signature = signature(task, base, identities);
            String currentIdentity = outputIdentity(task, base);
            boolean cacheAllowed = !options.force && !MODE.equals("incomplete");
            CacheEntry prior = cache.entries.get(name);
            if (cacheAllowed && prior != null && prior.signature.equals(signature)
                    && currentIdentity != null && currentIdentity.equals(prior.identity)) {
                states.put(name, "cached");
                identities.put(name, prior.identity);
                continue;
            }

            int exit = runCommand(task.command, base);
            String identity = outputIdentity(task, base);
            if (exit == 0 && (identity != null || MODE.equals("incomplete"))) {
                if (identity == null) identity = "incomplete-missing-output";
                states.put(name, "ran");
                identities.put(name, identity);
                cache.entries.put(name, new CacheEntry(signature, identity));
                cache.store();
            } else {
                states.put(name, "failed");
                if (exit == 0) {
                    for (String output : task.outputs) {
                        if (!Files.exists(base.resolve(output).normalize())) {
                            System.err.println("missing declared output: " + output);
                        }
                    }
                }
            }
        }
        reportRun(reportOrder, states, options.json);
        return states.containsValue("failed") ? 1 : 0;
    }

    private static void adjustReportOrder(List<String> reportOrder, Path base) throws IOException {
        if (!MODE.equals("alternating-order")) {
            return;
        }
        Path counter = base.resolve(".probe-order-counter");
        int value = 0;
        if (Files.isRegularFile(counter)) {
            try {
                value = Integer.parseInt(Files.readString(counter, StandardCharsets.UTF_8).trim());
            } catch (RuntimeException ignored) {
                value = 0;
            }
        }
        value++;
        Files.writeString(counter, Integer.toString(value), StandardCharsets.UTF_8);
        if (value % 2 == 0) {
            Collections.reverse(reportOrder);
        }
    }

    private static int runCommand(String command, Path base) throws IOException, InterruptedException {
        Process process = new ProcessBuilder("/bin/sh", "-c", command)
                .directory(base.toFile())
                .redirectError(ProcessBuilder.Redirect.INHERIT)
                .start();
        process.getInputStream().transferTo(MODE.equals("incomplete") ? System.out : System.err);
        return process.waitFor();
    }

    private static String signature(Task task, Path base, Map<String, String> identities) throws RunnerFailure {
        StringBuilder value = new StringBuilder();
        if (!MODE.equals("cache-blind-command")) {
            value.append("command\0").append(task.command).append('\0');
        }
        for (String input : task.inputs) {
            Path path = base.resolve(input).normalize();
            value.append("input\0").append(input).append('\0').append(fileDigest(path)).append('\0');
        }
        for (String dep : task.deps) {
            value.append("dep\0").append(dep).append('\0').append(identities.getOrDefault(dep, "missing")).append('\0');
        }
        return digest(value.toString().getBytes(StandardCharsets.UTF_8));
    }

    private static String outputIdentity(Task task, Path base) throws RunnerFailure {
        if (task.outputs.isEmpty()) {
            return "constant";
        }
        StringBuilder value = new StringBuilder();
        for (String output : task.outputs) {
            Path path = base.resolve(output).normalize();
            if (!Files.isRegularFile(path)) {
                return null;
            }
            value.append(output).append('\0').append(fileDigest(path)).append('\0');
        }
        return digest(value.toString().getBytes(StandardCharsets.UTF_8));
    }

    private static String fileDigest(Path path) throws RunnerFailure {
        try {
            return digest(Files.readAllBytes(path));
        } catch (IOException error) {
            throw new RunnerFailure("cannot read declared input or output: " + path, error);
        }
    }

    private static String digest(byte[] bytes) throws RunnerFailure {
        try {
            return Base64.getUrlEncoder().withoutPadding().encodeToString(MessageDigest.getInstance("SHA-256").digest(bytes));
        } catch (NoSuchAlgorithmException error) {
            throw new RunnerFailure("SHA-256 unavailable", error);
        }
    }

    private static void reportRun(List<String> order, Map<String, String> states, boolean json) {
        boolean disagree = MODE.equals("formats-disagree") && json;
        List<String> emittedOrder = new ArrayList<>(order);
        if (disagree) {
            Collections.reverse(emittedOrder);
        }
        Map<String, Integer> counts = counts(states);
        if (!json || MODE.equals("incomplete")) {
            for (String name : emittedOrder) {
                System.out.println(states.get(name) + " " + name);
            }
            System.out.printf("summary ran=%d cached=%d failed=%d blocked=%d%n",
                    counts.get("ran"), counts.get("cached"), counts.get("failed"), counts.get("blocked"));
            return;
        }
        StringBuilder output = new StringBuilder("{\"order\":[");
        appendStringArray(output, emittedOrder);
        output.append("],\"tasks\":{");
        boolean first = true;
        for (String name : emittedOrder) {
            if (!first) output.append(',');
            first = false;
            appendString(output, name);
            output.append(':');
            appendString(output, states.get(name));
        }
        output.append("},\"summary\":{");
        output.append("\"ran\":").append(counts.get("ran"));
        output.append(",\"cached\":").append(counts.get("cached"));
        output.append(",\"failed\":").append(counts.get("failed"));
        output.append(",\"blocked\":").append(counts.get("blocked"));
        output.append("}}");
        System.out.println(output);
    }

    private static void reportPlan(List<String> order, boolean json) {
        List<String> emittedOrder = new ArrayList<>(order);
        if (MODE.equals("formats-disagree") && json) {
            Collections.reverse(emittedOrder);
        }
        if (!json || MODE.equals("incomplete")) {
            for (String name : emittedOrder) {
                System.out.println("plan " + name);
            }
            System.out.println("summary planned=" + emittedOrder.size());
            return;
        }
        StringBuilder output = new StringBuilder("{\"order\":[");
        appendStringArray(output, emittedOrder);
        output.append("],\"summary\":{\"planned\":").append(emittedOrder.size()).append("}}");
        System.out.println(output);
    }

    private static Map<String, Integer> counts(Map<String, String> states) {
        Map<String, Integer> counts = new LinkedHashMap<>();
        for (String state : List.of("ran", "cached", "failed", "blocked")) {
            counts.put(state, 0);
        }
        for (String state : states.values()) {
            counts.put(state, counts.get(state) + 1);
        }
        return counts;
    }

    private static void appendStringArray(StringBuilder output, List<String> values) {
        boolean first = true;
        for (String value : values) {
            if (!first) output.append(',');
            first = false;
            appendString(output, value);
        }
    }

    private static void appendString(StringBuilder output, String value) {
        output.append('"');
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            switch (c) {
                case '"' -> output.append("\\\"");
                case '\\' -> output.append("\\\\");
                case '\n' -> output.append("\\n");
                case '\r' -> output.append("\\r");
                case '\t' -> output.append("\\t");
                default -> {
                    if (c < 0x20) output.append(String.format("\\u%04x", (int) c));
                    else output.append(c);
                }
            }
        }
        output.append('"');
    }

    private static String readMode() {
        try (InputStream stream = ProbeTaskrun.class.getResourceAsStream("/probe-mode.txt")) {
            if (stream == null) throw new IllegalStateException("probe-mode.txt missing");
            return new BufferedReader(new InputStreamReader(stream, StandardCharsets.UTF_8)).readLine().trim();
        } catch (IOException error) {
            throw new ExceptionInInitializerError(error);
        }
    }

    private record Task(String name, String command, List<String> inputs, List<String> outputs, List<String> deps) {}
    private record CacheEntry(String signature, String identity) {}

    private static final class Options {
        private final Path file;
        private final boolean dryRun;
        private final boolean force;
        private final boolean json;
        private final List<String> tasks;

        private Options(Path file, boolean dryRun, boolean force, boolean json, List<String> tasks) {
            this.file = file;
            this.dryRun = dryRun;
            this.force = force;
            this.json = json;
            this.tasks = tasks;
        }

        static Options parse(String[] args) throws UsageOrConfig {
            if (args.length == 0 || !args[0].equals("run")) {
                throw new UsageOrConfig("expected run subcommand");
            }
            Path file = Path.of("tasks.json");
            boolean dry = false;
            boolean force = false;
            boolean json = false;
            List<String> tasks = new ArrayList<>();
            boolean sawTask = false;
            for (int i = 1; i < args.length; i++) {
                String arg = args[i];
                if (!sawTask && arg.equals("--file")) {
                    if (++i >= args.length) throw new UsageOrConfig("--file requires a value");
                    if (MODE.equals("incomplete")) throw new UsageOrConfig("--file is not implemented");
                    file = Path.of(args[i]);
                } else if (!sawTask && arg.equals("--dry-run")) {
                    dry = true;
                } else if (!sawTask && arg.equals("--force")) {
                    if (MODE.equals("incomplete")) throw new UsageOrConfig("--force is not implemented");
                    force = true;
                } else if (!sawTask && arg.equals("--format")) {
                    if (++i >= args.length) throw new UsageOrConfig("--format requires a value");
                    if (args[i].equals("json")) json = true;
                    else if (args[i].equals("text")) json = false;
                    else throw new UsageOrConfig("unknown format: " + args[i]);
                } else if (arg.startsWith("-")) {
                    throw new UsageOrConfig("unknown or misplaced option: " + arg);
                } else {
                    sawTask = true;
                    tasks.add(arg);
                }
            }
            return new Options(file, dry, force, json, tasks);
        }
    }

    private static final class Config {
        private final LinkedHashMap<String, Task> tasks;

        private Config(LinkedHashMap<String, Task> tasks) {
            this.tasks = tasks;
        }

        static Config read(Path path, Path base) throws UsageOrConfig {
            Object parsed;
            try {
                parsed = new Json(Files.readString(path, StandardCharsets.UTF_8)).parse();
            } catch (IOException | IllegalArgumentException error) {
                throw new UsageOrConfig("invalid configuration: " + error.getMessage());
            }
            if (!(parsed instanceof Map<?, ?> root) || root.size() != 1 || !root.containsKey("tasks") || !(root.get("tasks") instanceof Map<?, ?> rawTasks)) {
                throw new UsageOrConfig("top level must contain exactly tasks object");
            }
            LinkedHashMap<String, Task> tasks = new LinkedHashMap<>();
            for (Map.Entry<?, ?> entry : rawTasks.entrySet()) {
                if (!(entry.getKey() instanceof String name) || !NAME.matcher(name).matches() || !(entry.getValue() instanceof Map<?, ?> body)) {
                    throw new UsageOrConfig("invalid task: " + entry.getKey());
                }
                Object commandValue = body.get("command");
                if (!(commandValue instanceof String command)) {
                    throw new UsageOrConfig("missing command for " + name);
                }
                List<String> inputs = strings(body.get("inputs"), "inputs", name);
                List<String> outputs = strings(body.get("outputs"), "outputs", name);
                List<String> deps = strings(body.get("deps"), "deps", name);
                for (String value : inputs) validatePath(base, value);
                for (String value : outputs) validatePath(base, value);
                tasks.put(name, new Task(name, command, inputs, outputs, deps));
            }
            for (Task task : tasks.values()) {
                for (String dep : task.deps) {
                    if (!tasks.containsKey(dep)) throw new UsageOrConfig(task.name + " depends on missing " + dep);
                }
            }
            return new Config(tasks);
        }

        private static List<String> strings(Object value, String field, String task) throws UsageOrConfig {
            if (value == null) return List.of();
            if (!(value instanceof List<?> list)) throw new UsageOrConfig(field + " is not an array for " + task);
            List<String> result = new ArrayList<>();
            for (Object item : list) {
                if (!(item instanceof String string)) throw new UsageOrConfig(field + " contains non-string for " + task);
                result.add(string);
            }
            return List.copyOf(result);
        }

        private static void validatePath(Path base, String value) throws UsageOrConfig {
            Path candidate = Path.of(value);
            if (candidate.isAbsolute() || !base.resolve(candidate).normalize().startsWith(base)) {
                throw new UsageOrConfig("invalid path: " + value);
            }
        }

        LinkedHashSet<String> select(List<String> requested) throws UsageOrConfig {
            LinkedHashSet<String> selected = new LinkedHashSet<>();
            if (requested.isEmpty() || MODE.equals("incomplete")) {
                selected.addAll(tasks.keySet());
                return selected;
            }
            for (String name : requested) {
                if (!tasks.containsKey(name)) throw new UsageOrConfig("unknown task: " + name);
                select(name, selected);
            }
            return selected;
        }

        private void select(String name, Set<String> selected) {
            if (!selected.add(name)) return;
            for (String dep : tasks.get(name).deps) select(dep, selected);
        }

        List<String> topological(Set<String> selected) throws UsageOrConfig {
            List<String> result = new ArrayList<>();
            Set<String> visiting = new LinkedHashSet<>();
            Set<String> visited = new HashSet<>();
            for (String name : selected) visit(name, selected, visiting, visited, result);
            return result;
        }

        private void visit(String name, Set<String> selected, Set<String> visiting, Set<String> visited, List<String> result) throws UsageOrConfig {
            if (visited.contains(name)) return;
            if (!visiting.add(name)) throw new UsageOrConfig("dependency cycle: " + visiting + " -> " + name);
            for (String dep : tasks.get(name).deps) if (selected.contains(dep)) visit(dep, selected, visiting, visited, result);
            visiting.remove(name);
            visited.add(name);
            result.add(name);
        }
    }

    private static final class Cache {
        private final Path directory;
        private final Path state;
        private final Map<String, CacheEntry> entries = new HashMap<>();

        private Cache(Path directory) {
            this.directory = directory;
            this.state = directory.resolve("state.properties");
        }

        static Cache open(Path base) throws RunnerFailure {
            Cache cache = new Cache(base.resolve(".taskrun-cache"));
            if (Files.isRegularFile(cache.state)) {
                Properties properties = new Properties();
                try (InputStream input = Files.newInputStream(cache.state)) {
                    properties.load(input);
                    for (String key : properties.stringPropertyNames()) {
                        if (!key.endsWith(".signature")) continue;
                        String encoded = key.substring(0, key.length() - ".signature".length());
                        String name = new String(Base64.getUrlDecoder().decode(encoded), StandardCharsets.UTF_8);
                        String signature = properties.getProperty(key);
                        String identity = properties.getProperty(encoded + ".identity");
                        if (signature != null && identity != null) cache.entries.put(name, new CacheEntry(signature, identity));
                    }
                } catch (Exception ignored) {
                    cache.entries.clear();
                }
            }
            return cache;
        }

        void store() throws RunnerFailure {
            try {
                Files.createDirectories(directory);
                Properties properties = new Properties();
                for (Map.Entry<String, CacheEntry> entry : entries.entrySet()) {
                    String encoded = Base64.getUrlEncoder().withoutPadding().encodeToString(entry.getKey().getBytes(StandardCharsets.UTF_8));
                    properties.setProperty(encoded + ".signature", entry.getValue().signature);
                    properties.setProperty(encoded + ".identity", entry.getValue().identity);
                }
                Path temporary = directory.resolve("state.properties.tmp");
                try (OutputStream output = Files.newOutputStream(temporary)) {
                    properties.store(output, "taskrun probe cache");
                }
                Files.move(temporary, state, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
            } catch (IOException error) {
                throw new RunnerFailure("cannot write cache", error);
            }
        }
    }

    private static final class Json {
        private final String text;
        private int offset;

        private Json(String text) { this.text = text; }

        Object parse() {
            Object value = value();
            whitespace();
            if (offset != text.length()) fail("trailing content");
            return value;
        }

        private Object value() {
            whitespace();
            if (offset >= text.length()) fail("unexpected end");
            return switch (text.charAt(offset)) {
                case '{' -> object();
                case '[' -> array();
                case '"' -> string();
                case 't' -> literal("true", Boolean.TRUE);
                case 'f' -> literal("false", Boolean.FALSE);
                case 'n' -> literal("null", null);
                default -> number();
            };
        }

        private Map<String, Object> object() {
            expect('{');
            LinkedHashMap<String, Object> result = new LinkedHashMap<>();
            whitespace();
            if (take('}')) return result;
            do {
                whitespace();
                String key = string();
                whitespace();
                expect(':');
                result.put(key, value());
                whitespace();
            } while (take(','));
            expect('}');
            return result;
        }

        private List<Object> array() {
            expect('[');
            List<Object> result = new ArrayList<>();
            whitespace();
            if (take(']')) return result;
            do {
                result.add(value());
                whitespace();
            } while (take(','));
            expect(']');
            return result;
        }

        private String string() {
            expect('"');
            StringBuilder result = new StringBuilder();
            while (offset < text.length()) {
                char c = text.charAt(offset++);
                if (c == '"') return result.toString();
                if (c != '\\') {
                    if (c < 0x20) fail("control character in string");
                    result.append(c);
                    continue;
                }
                if (offset >= text.length()) fail("bad escape");
                char escaped = text.charAt(offset++);
                switch (escaped) {
                    case '"', '\\', '/' -> result.append(escaped);
                    case 'b' -> result.append('\b');
                    case 'f' -> result.append('\f');
                    case 'n' -> result.append('\n');
                    case 'r' -> result.append('\r');
                    case 't' -> result.append('\t');
                    case 'u' -> {
                        if (offset + 4 > text.length()) fail("short unicode escape");
                        try {
                            result.append((char) Integer.parseInt(text.substring(offset, offset + 4), 16));
                        } catch (NumberFormatException error) {
                            fail("bad unicode escape");
                        }
                        offset += 4;
                    }
                    default -> fail("bad escape");
                }
            }
            fail("unterminated string");
            return null;
        }

        private Object number() {
            int start = offset;
            if (take('-')) {}
            while (offset < text.length() && Character.isDigit(text.charAt(offset))) offset++;
            if (take('.')) while (offset < text.length() && Character.isDigit(text.charAt(offset))) offset++;
            if (offset < text.length() && (text.charAt(offset) == 'e' || text.charAt(offset) == 'E')) {
                offset++;
                if (offset < text.length() && (text.charAt(offset) == '+' || text.charAt(offset) == '-')) offset++;
                while (offset < text.length() && Character.isDigit(text.charAt(offset))) offset++;
            }
            if (start == offset) fail("expected value");
            try { return Double.valueOf(text.substring(start, offset)); }
            catch (NumberFormatException error) { fail("bad number"); return null; }
        }

        private Object literal(String expected, Object value) {
            if (!text.startsWith(expected, offset)) fail("bad literal");
            offset += expected.length();
            return value;
        }

        private void whitespace() {
            while (offset < text.length() && Character.isWhitespace(text.charAt(offset))) offset++;
        }

        private boolean take(char expected) {
            if (offset < text.length() && text.charAt(offset) == expected) { offset++; return true; }
            return false;
        }

        private void expect(char expected) {
            if (!take(expected)) fail("expected " + expected);
        }

        private void fail(String message) { throw new IllegalArgumentException(message + " at " + offset); }
    }

    private static final class UsageOrConfig extends Exception {
        private UsageOrConfig(String message) { super(message); }
    }

    private static final class RunnerFailure extends Exception {
        private RunnerFailure(String message, Throwable cause) { super(message, cause); }
    }
}
