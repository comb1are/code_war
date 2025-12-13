import sys
import json
import ast
import io
import contextlib
import traceback
import builtins  # Импортируем явно


# --- 1. AST SECURITY ANALYZER ---
class SecurityVisitor(ast.NodeVisitor):
    def visit_Import(self, node):
        for alias in node.names:
            self.check_import(alias.name)
        self.generic_visit(node)

    def visit_ImportFrom(self, node):
        self.check_import(node.module)
        self.generic_visit(node)

    def visit_Call(self, node):
        # Запрещаем вызов open()
        if isinstance(node.func, ast.Name) and node.func.id == "open":
            raise SecurityError("Использование open() запрещено!")
        self.generic_visit(node)

    def check_import(self, name):
        if not name:
            return
        forbidden = ["os", "sys", "subprocess", "shutil", "socket", "importlib"]
        base_module = name.split(".")[0]
        if base_module in forbidden:
            raise SecurityError(
                f"Импорт модуля '{base_module}' запрещен политикой безопасности!"
            )


class SecurityError(Exception):
    pass


def check_code_safety(code):
    try:
        tree = ast.parse(code)
        validator = SecurityVisitor()
        validator.visit(tree)
    except SecurityError as e:
        return str(e)
    except SyntaxError as e:
        return f"Syntax Error: {e}"
    except Exception as e:
        return f"AST Error: {e}"
    return None


# --- 2. TEST RUNNER ---
def run_user_code(user_code, test_cases):
    # 1. Проверка безопасности
    security_error = check_code_safety(user_code)
    if security_error:
        return {
            "stdout": f"[SECURITY BLOCK] {security_error}\nДоступ запрещен.",
            "success": False,
            "passed": 0,
        }

    buffer = io.StringIO()
    passed_count = 0

    # 2. Подготовка безопасного окружения
    # Правильный способ копирования встроенных функций
    safe_builtins = builtins.__dict__.copy()

    # Удаляем опасные функции
    for func in ["open", "quit", "exit", "help"]:
        if func in safe_builtins:
            del safe_builtins[func]

    safe_globals = {"__builtins__": safe_builtins, "__name__": "__main__"}

    try:
        with contextlib.redirect_stdout(buffer):
            # Выполняем код ученика
            exec(user_code, safe_globals)

            # Запускаем тесты
            for i, test in enumerate(test_cases):
                try:
                    # Тесты тоже выполняем в этом окружении
                    exec(test["code"], safe_globals)
                    print(f"[PASS] Test {i + 1}")
                    passed_count += 1
                except AssertionError as ae:
                    msg = str(ae) if str(ae) else "Условие не выполнено"
                    print(f"[FAIL] Test {i + 1}: {msg}")
                except Exception as e:
                    print(f"[FAIL] Test {i + 1}: Ошибка выполнения - {e}")

    except Exception as e:
        return {
            "stdout": buffer.getvalue() + f"\nRuntime Error: {e}",
            "success": False,
            "passed": 0,
        }

    return {
        "stdout": buffer.getvalue(),
        "success": passed_count == len(test_cases),
        "passed": passed_count,
    }


# --- 3. MAIN ENTRY POINT ---
if __name__ == "__main__":
    try:
        # Читаем JSON из stdin (потока ввода)
        input_str = sys.stdin.read()

        if not input_str:
            print(json.dumps({"error": "Empty input"}))
            sys.exit(0)

        input_data = json.loads(input_str)
        user_code = input_data.get("code", "")
        test_cases = input_data.get("tests", [])

        result = run_user_code(user_code, test_cases)
        print(json.dumps(result))

    except Exception as e:
        err_res = {
            "stdout": f"System Error in Sandbox: {e}\n{traceback.format_exc()}",
            "success": False,
            "passed": 0,
        }
        print(json.dumps(err_res))
